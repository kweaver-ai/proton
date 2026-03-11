import paramiko
import pymysql

class SSHClient:
    def __init__(self, host, port, user, password):
        self.client = paramiko.SSHClient()
        self.client.load_system_host_keys()
        self.client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        self.client.connect(hostname=host, port=port, username=user, password=password, timeout=10)
    def exec_cmd(self, cmd_str, timeout=30):
        stdin, stdout, stderr = self.client.exec_command(cmd_str, timeout=timeout)
        out, err = stdout.read().decode(), stderr.read().decode()
        # print(f"Exec cmd {cmd_str}, out: {out}, err: {err}")
        return out, err
    def scp(self, src, dst):
        c = self.client.open_sftp()
        c.put(localpath=src, remotepath=dst)
        c.close()
    def __del__(self):
        self.client.close()

class DBClient:
    def __init__(self, host, port, user, password):
        self.db = pymysql.connect(
            host=host,
            port=port,
            user=user,
            password=password,
        )
        self.db.autocommit(1)
    def query(self, sql, args):
        with self.db.cursor() as cursor:
            cursor.execute(sql, args)
            result = cursor.fetchall()
            return result
    def exec(self, sql, args):
        with self.db.cursor() as cursor:
            cursor.execute(sql, args)
    def is_health(self):
        return self.query("select 1", ())
    def __del__(self):
        self.db.close()

