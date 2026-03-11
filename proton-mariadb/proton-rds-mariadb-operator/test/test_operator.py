import re
import pytest
import allure
import common
import json
import time
import requests

SSH_CLIENT = None

class Test_Operator:
    def wait_cr_ready(self):
        for i in range(60):
            stdout, stderr = SSH_CLIENT.exec_cmd(
                cmd_str="kubectl get pod | grep proton-rds | grep 0/1",
                timeout=30
            )

            assert not re.search(r'.*CrashLoopBackOff.*', stdout)
            if stdout == "":
                break
            time.sleep(5)
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl get pod | grep proton-rds | grep 0/1",
            timeout=30
            )
        assert stdout == ""

    def generate_nodeport_json(self, name, label, port, targetPort=3306):
        data = {
            "kind": "Service",
            "apiVersion": "v1",
            "metadata": {
                "name": name,
                "namespace": "default",
            },
            "spec":{
                "selector":label,
                "type": "NodePort",
                "ports": [
                    {
                    "name": "mariadb",
                    "port": targetPort,
                    "targetPort": targetPort,
                    "nodePort": port,
                    },
                ]
            }
        }
        return data

    def generate_cr_json(self, host, n):
        volume = []
        for i in range(n):
            volume.append(
                {
                    "host": host,
                    "path": f"/data/pv-{i}"
                }
            )
        data = {
            "apiVersion": "rds.proton.aishu.cn/v1",
            "kind": "RDSMariaDBCluster",
            "metadata": {
                "name": "proton-rds",
                "namespace": "default",
            },
            "spec":{
                "secretName": "rds-secret",
                "replicas": n,
                "etcd": {
                    "image": "acr.aishu.cn/proton/etcd:v3.3.19",
                    "imagePullPolicy": "IfNotPresent",
                },
                "exporter": {
                    "image": "acr.aishu.cn/proton/rds-exporter:2.0.1-develop",
                    "imagePullPolicy": "IfNotPresent",
                },
                "mgmt": {
                    "image": "acr.aishu.cn/proton/rds-mgmt:2.1.0-develop",
                    "imagePullPolicy": "IfNotPresent",
                    "conf": {
                        "lang": "zh_CN",
                        "logLevel": "error",
                    },
                    "service": {
                        "enableDualStack": False,
                        "port": 8888,
                    },
                    "resources": {
                    }
                },
                "mariadb": {
                    "image": "acr.aishu.cn/proton/rds-mariadb:2.0.1-develop",
                    "imagePullPolicy": "IfNotPresent",
                    "conf": {
                        "wait_timeout": 3600,
                        "innodb_buffer_pool_size": "8G",
                    },
                    "service": {
                        "enableDualStack": False,
                        "port": 3306,
                    },
                    "storage": {
                        "capacity": "10Gi",
                        "storageClassName": "",
                        "volumeSpec": volume,
                    },
                    "nodeAffinity": {
                        "requiredDuringSchedulingIgnoredDuringExecution": {
                            "nodeSelectorTerms": [
                                {
                                    "matchExpressions": [
                                        {
                                            "key": "role",
                                            "operator": "In",
                                            "values": [
                                                "maraidb",
                                            ]
                                        }
                                    ]
                                }
                            ]
                        }
                    },
                    "resources": {},
                },
            }
        }
        return data

    @allure.title("准备环境")
    def setup_class(self):
        pass

    @allure.title("清理环境")
    def teardown_class(self):
        SSH_CLIENT.exec_cmd(
            cmd_str="kubectl delete RDSMariaDBCluster/proton-rds",
            timeout=120
        )
        SSH_CLIENT.exec_cmd(
            cmd_str="helm delete --purge operator",
            timeout=30
        )
        SSH_CLIENT.exec_cmd(
            cmd_str="kubectl delete secret/rds-secret",
            timeout=30
        )
        SSH_CLIENT.exec_cmd(
            cmd_str="rm -rf /data/pv-{0,1,2}",
            timeout=30
        )
        SSH_CLIENT.exec_cmd("kubectl delete -f /tmp/svc.json")
        SSH_CLIENT.exec_cmd("kubectl delete -f /tmp/mgmt.json")

    @allure.title("安装Operator")
    def test_0(self,host,user,password,CRDVersion):
        global SSH_CLIENT
        SSH_CLIENT = common.SSHClient(host, 22, user, password)
        SSH_CLIENT.exec_cmd(
            cmd_str="mkdir -p /data/pv-{0,1,2}",
            timeout=30
        )
        SSH_CLIENT.exec_cmd(
            cmd_str="helm repo add proton https://acr.aishu.cn/chartrepo/proton",
            timeout=30
        )
        SSH_CLIENT.exec_cmd(
            cmd_str="helm repo update",
            timeout=30
        )
        SSH_CLIENT.exec_cmd(
            cmd_str=f"helm install --name operator proton/rds-mariadb-operator --version {CRDVersion} --wait",
            timeout=120
        )
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl get pod -n proton-rds-mariadb-operator-system",
            timeout=30
        )

        assert re.search(r'.*2/2.*', stdout)

    @allure.title("安装1副本cr成功")
    def test_1(self, host):
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="hostname",
            timeout=30
        )
        hostname = stdout.replace("\n", "")

        SSH_CLIENT.exec_cmd(
            cmd_str="kubectl create secret generic rds-secret --from-literal=username=root --from-literal=password=fakepassword",
            timeout=30
        )

        data = self.generate_cr_json(hostname, 1)
        with open('cr.json', 'w') as f:
            json.dump(data, f)
        SSH_CLIENT.scp('cr.json', '/tmp/cr.json')

        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl create -f /tmp/cr.json",
            timeout=30
        )
        assert re.search(r'.*created.*', stdout)

        time.sleep(30)
        self.wait_cr_ready()
        data = self.generate_nodeport_json(
            "proton-rds-mariadb-nodeport",
            {
                "statefulset.kubernetes.io/pod-name":"proton-rds-mariadb-0",
            },
            30036
        )
        with open('svc.json', 'w') as f:
            json.dump(data, f)
        SSH_CLIENT.scp('svc.json', '/tmp/svc.json')
        SSH_CLIENT.exec_cmd("kubectl create -f /tmp/svc.json")

        SSH_CLIENT.exec_cmd(
            cmd_str="firewall-cmd --add-port=30036/tcp",
            timeout=30
        )
        db = common.DBClient(host, 30036, "root", "fakepassword")
        db.is_health()

    @allure.title("1副本扩容3副本成功/3副本缩容1副本成功")
    def test_2(self, host):
        db = common.DBClient(host, 30036, "root", "fakepassword")
        db.exec('create database if not exists test;',())
        db.exec('use test;',())
        db.exec('create table if not exists t1(id int not null,PRIMARY KEY (id));',())
        db.exec('replace into t1 values (%s);',(1,))
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="hostname",
            timeout=30
        )
        hostname = stdout.replace("\n", "")

        data = self.generate_cr_json(hostname, 3)
        with open('cr.json', 'w') as f:
            json.dump(data, f)
        SSH_CLIENT.scp('cr.json', '/tmp/cr.json')

        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl apply -f /tmp/cr.json",
            timeout=30
        )
        assert re.search(r'.*configured.*', stdout)

        time.sleep(60)
        self.wait_cr_ready()
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl get pod | grep proton-rds",
            timeout=30
        )
        db = common.DBClient(host, 30036, "root", "fakepassword")
        result = db.query("select id from test.t1", ())
        assert result[0][0] == 1

        data = self.generate_cr_json(hostname, 1)
        with open('cr.json', 'w') as f:
            json.dump(data, f)
        SSH_CLIENT.scp('cr.json', '/tmp/cr.json')
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl apply -f /tmp/cr.json",
            timeout=30
        )
        assert re.search(r'.*configured.*', stdout)
        time.sleep(60)
        self.wait_cr_ready()
        db = common.DBClient(host, 30036, "root", "fakepassword")
        result = db.query("select id from test.t1", ())
        assert result[0][0] == 1

        data = self.generate_cr_json(hostname, 3)
        with open('cr.json', 'w') as f:
            json.dump(data, f)
        SSH_CLIENT.scp('cr.json', '/tmp/cr.json')
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl apply -f /tmp/cr.json",
            timeout=30
        )
        assert re.search(r'.*configured.*', stdout)
        time.sleep(60)
        self.wait_cr_ready()
        db = common.DBClient(host, 30036, "root", "fakepassword")
        result = db.query("select id from test.t1", ())
        assert result[0][0] == 1

    @allure.title("1副本停机服务可用, 可以自动恢复")
    def test_3(self, host):
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl delete pod/proton-rds-mariadb-2",
            timeout=30
        )
        db = common.DBClient(host, 30036, "root", "fakepassword")
        db.is_health()
        self.wait_cr_ready()
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl get pod | grep proton-rds",
            timeout=30
        )

        db.is_health()

    @allure.title("3副本停机,自动恢复后服务可用")
    def test_4(self, host):
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl delete pod proton-rds-mariadb-0  proton-rds-mariadb-1  proton-rds-mariadb-2",
            timeout=60
        )
        time.sleep(60)
        self.wait_cr_ready()
        stdout, stderr = SSH_CLIENT.exec_cmd(
            cmd_str="kubectl get pod | grep proton-rds",
            timeout=30
        )
        db = common.DBClient(host, 30036, "root", "fakepassword")
        db.is_health()

    @allure.title("数据库管理")
    def test_5(self, host):
        data = self.generate_nodeport_json(
            "proton-rds-mgmt-nodeport",
            {
                "app":"proton-rds-mgmt",
            },
            30037,
            8888,
        )
        with open('mgmt.json', 'w') as f:
            json.dump(data, f)
        SSH_CLIENT.scp('mgmt.json', '/tmp/mgmt.json')
        SSH_CLIENT.exec_cmd("kubectl create -f /tmp/mgmt.json")
        SSH_CLIENT.exec_cmd(
            cmd_str="firewall-cmd --add-port=30037/tcp",
            timeout=30
        )

        resp = requests.put(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/dbs/ecms',
            headers={
                "Content-Type": "application/json",
                "admin-key":"xxxx",
            },
            data=json.dumps(
                {
                "charset":"utf8mb4"
                }
            )
        )
        assert resp.status_code == 400

        resp = requests.put(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/dbs/ecms',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
            data=json.dumps(
                {
                "charset":"utf8mb4"
                }
            )
        )
        assert resp.status_code == 201
        db = common.DBClient(host, 30036, "root", "fakepassword")
        result = db.query("show databases", ())
        assert ('ecms', ) in result

        resp = requests.put(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/dbs/ecms',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
            data=json.dumps(
                {
                "charset":"utf8mb4"
                }
            )
        )
        assert resp.status_code == 403

        resp = requests.get(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/dbs',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
        )
        assert resp.status_code == 200
        dbs = [v['db_name'] for v in resp.json()]
        assert 'ecms' in dbs

        resp = requests.delete(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/dbs/ecms',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
        )
        assert resp.status_code == 204
        resp = requests.get(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/dbs',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
        )
        dbs = [v['db_name'] for v in resp.json()]
        assert 'ecms' not in dbs

    @allure.title("数据库账户管理")
    def test_6(self, host):
        resp = requests.put(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/users/anyshare',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
            data=json.dumps(
                {
                "password":"ZWlzb28uY29tMTIz"
                }
            )
        )

        assert resp.status_code == 200

        resp = requests.get(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/users',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
        )
        users = [v['username'] for v in resp.json()]
        assert 'anyshare' in users

        resp = requests.delete(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/users/anyshare',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
        )
        assert resp.status_code == 204
        resp = requests.get(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/users',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
        )
        users = [v['username'] for v in resp.json()]
        assert 'anyshare' not in users

    @allure.title("数据库账户权限管理")
    def test_7(self, host):
        resp = requests.put(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/users/anyshare',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
            data=json.dumps(
                {
                "password":"ZWlzb28uY29tMTIz"
                }
            )
        )
        resp = requests.put(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/users/anyshare/privileges',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
            data=json.dumps(
                [
                    {
                        "db_name":"test",
                        "privilege_type": "ReadWrite",
                    }
                ]
            )
        )
        assert resp.status_code == 204
        db = common.DBClient(host, 30036, "anyshare", "fakepassword")
        db.exec('use test;',())
        db.exec('create table if not exists t1(id int not null,PRIMARY KEY (id));',())
        db.exec('replace into t1 values (%s);',(1,))
        result = db.query("select id from test.t1", ())
        assert result[0][0] == 1

        resp = requests.put(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/dbs/ecms',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
            data=json.dumps(
                {
                "charset":"utf8mb4"
                }
            )
        )
        resp = requests.patch(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/users/anyshare/privileges',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
            data=json.dumps(
                [
                    {
                        "db_name":"ecms",
                        "privilege_type": "ReadWrite",
                    }
                ]
            )
        )
        assert resp.status_code == 204
        db = common.DBClient(host, 30036, "anyshare", "fakepassword")
        db.exec('use test;',())
        db.exec('create table if not exists t1(id int not null,PRIMARY KEY (id));',())
        db.exec('replace into t1 values (%s);',(1,))
        result = db.query("select id from test.t1", ())
        assert result[0][0] == 1
        db.exec('use ecms;',())
        db.exec('create table if not exists t1(id int not null,PRIMARY KEY (id));',())
        db.exec('replace into t1 values (%s);',(1,))
        result = db.query("select id from test.t1", ())
        assert result[0][0] == 1

    @allure.title("备份管理")
    def test_8(self, host):
        resp = requests.post(
                url=f'http://{host}:30037/api/proton-rds-mgmt/v2/backups',
                headers={
                    "Content-Type": "application/json",
                    "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
                },
                data=json.dumps(
                    {
                    "backup_dirxxx":"/data"
                    }
                )
            )
        assert resp.status_code == 400

        resp = requests.post(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/backups',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            },
            data=json.dumps(
                {
                "backup_dir":"/data"
                }
            )
        )
        assert resp.status_code == 202
        backup_id = resp.json()["id"]

        resp = requests.get(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/backups',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            }
        )
        assert resp.status_code == 200
        assert resp.json()[0]["id"] == backup_id

        resp = requests.delete(
            url=f'http://{host}:30037/api/proton-rds-mgmt/v2/backups/{backup_id}',
            headers={
                "Content-Type": "application/json",
                "admin-key":"cm9vdDplaXNvby5jb20xMjM=",
            }
        )
        assert resp.status_code == 204













