package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"proton-rds-mgmt/common"
	"strconv"
	"strings"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// 备份管理
//
//go:generate mockgen -package mock -source ./backup_mgmt.go -destination ./mock/mock_backup_mgmt.go
type BackupMgmt interface {
	SetConfig(config *common.Config)
	// CreateBackup 创建备份
	CreateBackup(string, string) (BackupInfo, error)
	// DeleteBackup 删除备份
	DeleteBackup(string) error
	// ListBackup 返回最新的备份包列表
	ListBackup() (BackupInfos, error)
	// GetBackupDataSize 获取备份需要的空间
	GetBackupDataSize() (int, error)
	// 创建恢复任务
	CreateRecovery(string) (RecStaus, error)
	// 获取恢复进度
	ListRecovery() (RecStaus, error)
	// 恢复数据后扩容
	ScaleHook() error
}

type backupMgmt struct {
	logger     common.Logger
	config     *common.Config
	httpClient common.HTTPClient
	k8sclient  *common.K8SClient
}

var (
	once sync.Once
	b    BackupMgmt
)

// NewRDSMgmt 创建数据库操作对象
func NewBackupMgmt() BackupMgmt {
	once.Do(func() {
		b = &backupMgmt{
			logger:     common.NewLogger(),
			httpClient: common.NewHTTPClient(),
		}
	})
	return b
}

// SetConfig 设置Config
func (b *backupMgmt) SetConfig(config *common.Config) {
	b.logger.Infoln("Set Config")
	b.config = config
	b.init()
}

func (b *backupMgmt) init() {
	k8sclient, err := common.NewK8SClient()
	if err != nil {
		b.logger.Fatalln(err)
	}
	b.k8sclient = k8sclient
}

// 创建备份
func (b *backupMgmt) CreateBackup(adminKey, path string) (BackupInfo, error) {
	b.logger.Debugln("rdsMgmt.CreateBackup")
	timeUnixMill := time.Now().UnixNano() / 1e6
	backupID := fmt.Sprint(timeUnixMill)
	packageName := filepath.Join(path, backupID+".gz")
	createTime := common.TimeStampToString(timeUnixMill)

	backupInfos, err := b.ListBackup()
	if err != nil {
		return BackupInfo{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}
	for _, value := range backupInfos {
		if value.Status == BackupStatusRunning {
			return BackupInfo{}, common.NewHTTPError(fmt.Sprintf("Backup %s is running...", value.Id), common.InternalError, nil)
		}
	}

	backupJob, err := b.constructBackupJob(backupID, path)
	if err != nil {
		return BackupInfo{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}
	_, err = b.k8sclient.CreateJob(&backupJob)
	if err != nil {
		return BackupInfo{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}
	return BackupInfo{
		Id:          backupID,
		PackageName: packageName,
		CreateTime:  createTime,
		Status:      BackupStatusRunning,
		StorageNode: backupJob.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"],
	}, nil
}

// 删除指定的备份:
func (b *backupMgmt) DeleteBackup(backupID string) error {
	b.logger.Debugln("rdsMgmt.DeleteBackup")
	job, err := b.k8sclient.ClientSet().
		BatchV1().
		Jobs(b.config.Namespace).
		Get(context.TODO(), "xb-"+backupID, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return common.NewHTTPError(err.Error(), common.BackupNotExist, nil)
		}
		return common.NewHTTPError(err.Error(), common.InternalError, nil)
	}
	deleteJob, _ := b.constructDeleteJob(
		job.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"],
		job.Spec.Template.Spec.Volumes[1].HostPath.Path,
		backupID,
		job.Spec.Template.Spec.Containers[0].Image,
	)
	err = b.k8sclient.DeleteJob(job.Namespace, job.Name)
	if err != nil {
		return common.NewHTTPError(err.Error(), common.InternalError, nil)
	}
	_, err = b.k8sclient.CreateJob(&deleteJob)
	if err != nil {
		return common.NewHTTPError(err.Error(), common.InternalError, nil)
	}
	go func() {
		time.Sleep(60 * time.Second)
		b.k8sclient.DeleteJob(deleteJob.Namespace, deleteJob.Name)
	}()
	return nil
}

// 获取备份列表
func (b *backupMgmt) ListBackup() (BackupInfos, error) {
	b.logger.Debugln("rdsMgmt.ListBackup")
	backupInfos := BackupInfos{}
	jobList, err := b.k8sclient.ClientSet().
		BatchV1().
		Jobs(b.config.Namespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: "xb",
		})
	if err != nil {
		return backupInfos, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}

	for _, job := range jobList.Items {
		s := strings.Split(job.Name, "-")
		if len(s) != 2 {
			continue
		}
		status := "Unknown"
		if job.Status.Succeeded > 0 {
			status = BackupStatusSuccess
		} else if job.Status.Failed > 0 {
			status = BackupStatusFailed
		} else {
			status = BackupStatusRunning
		}
		timestamp, _ := strconv.ParseInt(s[1], 10, 64)
		backup := BackupInfo{
			Id:          s[1],
			PackageName: filepath.Join(job.Spec.Template.Spec.Volumes[1].HostPath.Path, s[1]+".gz"),
			CreateTime:  common.TimeStampToString(timestamp),
			Status:      status,
			StorageNode: job.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"],
		}
		backupInfos = append(backupInfos, backup)
	}
	return backupInfos, nil
}

func (b *backupMgmt) constructBackupJob(backupID, backupDir string) (batchv1.Job, error) {
	isReady, _ := b.k8sclient.IsPodReady(b.config.Namespace, b.config.CRName+"-mariadb-0")
	if !isReady {
		return batchv1.Job{}, common.NewHTTPError("pod not ready, not supported backup", common.InternalError, nil)
	}

	pv, err := b.k8sclient.ClientSet().
		CoreV1().
		PersistentVolumes().
		Get(context.TODO(), b.config.Namespace+"-"+b.config.CRName+"-mariadb-pv-0", metav1.GetOptions{})
	if err != nil {
		return batchv1.Job{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}

	pod, err := b.k8sclient.ClientSet().
		CoreV1().
		Pods(b.config.Namespace).
		Get(context.TODO(), b.config.CRName+"-mariadb-0", metav1.GetOptions{})
	if err != nil {
		return batchv1.Job{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}
	secret := "rds-secret"
	for _, value := range pod.Spec.Containers[0].Env {
		if value.Name == "MYSQL_ROOT_USER" {
			secret = value.ValueFrom.SecretKeyRef.LocalObjectReference.Name
			break
		}
	}

	host := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
	path := pv.Spec.PersistentVolumeSource.Local.Path

	var backOffLimit int32 = 0
	return batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xb-" + backupID,
			Namespace: b.config.Namespace,
			Labels: map[string]string{
				"xb": "",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "mariadb",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: path,
								},
							},
						},
						{
							Name: "backup",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: backupDir,
								},
							},
						},
					},
					RestartPolicy: "Never",
					Containers: []corev1.Container{
						{
							Name:  "backup",
							Image: pod.Spec.Containers[0].Image,
							Env: []corev1.EnvVar{
								{
									Name:  "MYSQL_HOST",
									Value: b.config.CRName + "-mariadb-0." + b.config.CRName + "-mariadb",
								},
								{
									Name:  "MYSQL_PORT",
									Value: "3306",
								},
								{
									Name: "MYSQL_ROOT_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret,
											},
											Key: "username",
										},
									},
								},
								{
									Name: "MYSQL_ROOT_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret,
											},
											Key: "password",
										},
									},
								},
								{
									Name:  "BACKUP_ID",
									Value: backupID,
								},
							},
							Command: []string{
								"/bin/bash",
								"-ecx",
								"mariabackup  --datadir=/var/lib/mysql --host=$MYSQL_HOST --port=$MYSQL_PORT --user=$MYSQL_ROOT_USER --password=$MYSQL_ROOT_PASSWORD --parallel=16 --backup --stream=xbstream | gzip > /var/lib/backup/$BACKUP_ID.gz",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "mariadb",
									MountPath: "/var/lib/mysql",
								},
								{
									Name:      "backup",
									MountPath: "/var/lib/backup",
								},
							},
						},
					},
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": host,
					},
				},
			},
		},
	}, nil
}

func (b *backupMgmt) constructDeleteJob(host, path, backupID, image string) (batchv1.Job, error) {
	return batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "del-xb-" + backupID,
			Namespace: b.config.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "backup",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: path,
								},
							},
						},
					},
					RestartPolicy: "Never",
					Containers: []corev1.Container{
						{
							Name:  "del-backup",
							Image: image,
							Env: []corev1.EnvVar{
								{
									Name:  "BACKUP_ID",
									Value: backupID,
								},
							},
							Command: []string{
								"/bin/bash",
								"-ecx",
								"rm -rf /var/lib/backup/$BACKUP_ID.gz",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "backup",
									MountPath: "/var/lib/backup",
								},
							},
						},
					},
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": host,
					},
				},
			},
		},
	}, nil
}

// 没有找到更好的获取备份大小的方法
func (b *backupMgmt) GetBackupDataSize() (int, error) {
	stdout, _, err := b.k8sclient.ExecPod(b.config.Namespace,
		b.config.CRName+"-mariadb-0",
		"mariadb",
		[]string{
			"/bin/bash",
			"-c",
			"du -s /var/lib/mysql/* | grep -v mysql-bin | awk '{sum += $1};END {print sum}'",
		},
	)
	if err != nil {
		return 0, err
	}
	size_str := strings.Replace(stdout.String(), "\n", "", -1)
	size, err := strconv.Atoi(size_str)
	if err != nil {
		return 0, err
	}
	return size, nil
}

func (b *backupMgmt) CreateRecovery(backup string) (RecStaus, error) {
	b.logger.Debugln("rdsMgmt.CreateRecovery")

	// 不允许并行
	job, err := b.k8sclient.ClientSet().
		BatchV1().
		Jobs(b.config.Namespace).
		Get(context.TODO(), "xb-rec-"+b.config.CRName, metav1.GetOptions{})
	if err == nil {
		if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
			return RecStaus{}, common.NewHTTPError("Another restorations is running", common.RecTaskDenied, nil)
		}
		b.k8sclient.DeleteJob(job.Namespace, job.Name)
	}

	sts, err := b.k8sclient.ClientSet().
		AppsV1().
		StatefulSets(b.config.Namespace).
		Get(context.TODO(), b.config.CRName+"-mariadb", metav1.GetOptions{})
	if err != nil {
		return RecStaus{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}
	image := sts.Spec.Template.Spec.Containers[0].Image

	backupDir := filepath.Dir(backup)
	backupFile := filepath.Base(backup)
	pv, err := b.k8sclient.ClientSet().
		CoreV1().
		PersistentVolumes().
		Get(context.TODO(), b.config.Namespace+"-"+b.config.CRName+"-mariadb-pv-0", metav1.GetOptions{})
	if err != nil {
		return RecStaus{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}
	host := pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
	mariadbDir := pv.Spec.PersistentVolumeSource.Local.Path

	recoveryJob := b.constructRecoveryJob(host, mariadbDir, backupDir, backupFile, image)

	// 恢复时需要缩容mariadb
	err = b.k8sclient.ScaleSts(b.config.Namespace, b.config.CRName+"-mariadb", 0)
	if err != nil {
		return RecStaus{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}

	_, err = b.k8sclient.CreateJob(&recoveryJob)
	if err != nil {
		return RecStaus{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}

	return RecStaus{
		Status: RecStatusRunning,
		Msg:    "",
	}, nil
}

func (b *backupMgmt) ListRecovery() (RecStaus, error) {
	b.logger.Debugln("rdsMgmt.ListRecovery")
	job, err := b.k8sclient.ClientSet().
		BatchV1().
		Jobs(b.config.Namespace).
		Get(context.TODO(), "xb-rec-"+b.config.CRName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return RecStaus{}, nil
		}
		return RecStaus{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
	}

	if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
		return RecStaus{
			Status: RecStatusRunning,
			Msg:    "xb-rec job running.",
		}, nil
	} else if job.Status.Failed != 0 {
		return RecStaus{
			Status: RecStatusFailed,
			Msg:    "xb-rec job failed",
		}, nil
	} else {
		// 扩容mariadb
		sts, err := b.k8sclient.ClientSet().
			AppsV1().
			StatefulSets(b.config.Namespace).
			Get(context.TODO(), b.config.CRName+"-mariadb", metav1.GetOptions{})
		if err != nil {
			b.logger.Errorln(err)
			return RecStaus{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
		}

		if *sts.Spec.Replicas == 0 {
			err = b.k8sclient.ScaleSts(b.config.Namespace, b.config.CRName+"-mariadb", 1)
			if err != nil {
				b.logger.Errorln(err)
				return RecStaus{}, common.NewHTTPError(err.Error(), common.InternalError, nil)
			}
			return RecStaus{
				Status: RecStatusRunning,
				Msg:    "xb-rec job end, scaling",
			}, nil
		}

		isReady, err := b.k8sclient.IsPodReady(b.config.Namespace, b.config.CRName+"-mariadb-0")
		if err != nil {
			return RecStaus{}, err
		}
		if !isReady {
			return RecStaus{
				Status: RecStatusRunning,
				Msg:    "xb-rec job end, waiting for mariadb ready",
			}, nil
		} else {
			return RecStaus{
				Status: RecStatusSuccess,
				Msg:    "",
			}, nil
		}
	}

}

func (b *backupMgmt) ScaleHook() error {
	b.logger.Debugln("rdsMgmt.ScaleHook")

	_, err := b.k8sclient.ClientSet().
		AppsV1().
		StatefulSets(b.config.Namespace).
		Get(context.TODO(), b.config.CRName+"-mariadb", metav1.GetOptions{})
	if err != nil {
		b.logger.Errorln(err)
		return err
	}

	return b.k8sclient.ScaleSts(b.config.Namespace, b.config.CRName+"-mariadb", 1)

}

func (b *backupMgmt) constructRecoveryJob(host, mariadbDir, backupDir, backupFile, image string) batchv1.Job {
	var backOffLimit int32 = 0
	return batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xb-rec-" + b.config.CRName,
			Namespace: b.config.Namespace,
			Labels: map[string]string{
				"xb-rec": "",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "mariadb",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: mariadbDir,
								},
							},
						},
						{
							Name: "backup",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: backupDir,
								},
							},
						},
					},
					RestartPolicy: "Never",
					Containers: []corev1.Container{
						{
							Name:  "backup",
							Image: image,
							Env: []corev1.EnvVar{
								{
									Name:  "BACKUP_FILE",
									Value: backupFile,
								},
							},
							Command: []string{
								"/bin/bash",
								"-ecx",
								"rm -rf /var/lib/mysql/*; " +
									"rm -rf /var/lib/backup/tmp; " +
									"mkdir /var/lib/backup/tmp; " +
									"gunzip -c /var/lib/backup/$BACKUP_FILE | mbstream -x -C /var/lib/backup/tmp; " +
									"mariabackup --prepare --use-memory=1G --target-dir=/var/lib/backup/tmp; " +
									"mariabackup --move-back --target-dir=/var/lib/backup/tmp --datadir=/var/lib/mysql; " +
									fmt.Sprintf("curl --max-time 5 -X POST http://%s-mgmt-cluster:8888/api/proton-rds-mgmt/v2/scales_hook || exit 0", b.config.CRName),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "mariadb",
									MountPath: "/var/lib/mysql",
								},
								{
									Name:      "backup",
									MountPath: "/var/lib/backup",
								},
							},
						},
					},
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": host,
					},
				},
			},
		},
	}
}
