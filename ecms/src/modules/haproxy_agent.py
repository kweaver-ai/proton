#!/usr/bin/env python
#-*- coding: utf-8 -*-
"""
Python library for editing haproxy serverblocks.
"""

import re
import os
import time
from src.modules.pydeps import cmdprocess, filelib
from src.modules.pydeps.logger import syslog, syslog_cmd

INDENT = '    '

SERVICE_NAME = 'haproxy'
HAPROXY_CONFIG = "/usr/local/haproxy/haproxy.cfg"
MODULE_NAME = 'HAProxyAgent'


class Conf(object):
    """
    Represents an haproxy configuration.

    A `Conf` can consist of any number of server blocks, as well as Upstream
    and other types of containers. It can also include top-level comments.
    """

    def __init__(self, *args):
        """
        Initialize object.

        :param *args: Any objects to include in this Conf.
        """
        self.children = list(args)

    def add(self, *args):
        """
        Add object(s) to the Conf.

        :param *args: Any objects to add to the Conf.
        :returns: full list of Conf's child objects
        """
        self.children.extend(args)
        return self.children

    def remove(self, *args):
        """
        Remove object(s) from the Conf.

        :param *args: Any objects to remove from the Conf.
        :returns: full list of Conf's child objects
        """
        for x in args:
            self.children.remove(x)
        return self.children

    def filter(self, btype='', name=''):
        """
        Return child object(s) of this Conf that satisfy certain criteria.

        :param str btype: Type of object to filter by (e.g. 'Key')
        :param str name: Name of key OR container value to filter by
        :returns: full list of matching child objects
        """
        filtered = []
        for x in self.children:
            if name and isinstance(x, Key) and x.name == name:
                filtered.append(x)
            elif isinstance(x, Container) and x.__class__.__name__ == btype\
                    and x.value == name:
                filtered.append(x)
            elif not name and btype and x.__class__.__name__ == btype:
                filtered.append(x)
        return filtered

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        # return {'conf': [x.as_dict for x in self.children]}
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            if d.has_key('defaults') and new_dict.has_key('defaults'):
                new_dict['defaults'].update(d['defaults'])
            elif d.has_key('backend') and new_dict.has_key('backend'):
                new_dict['backend'].update(d['backend'])
            elif d.has_key('frontend') and new_dict.has_key('frontend'):
                new_dict['frontend'].update(d['frontend'])
            else:
                new_dict.update(d)

        return {'conf': new_dict}

    @property
    def as_strings(self):
        """Return the entire Conf as haproxy config strings."""
        ret = []
        for x in self.children:
            for y in x.as_strings:
                ret.append(y)
        if ret:
            ret[-1] = re.sub('}\n+$', '}\n', ret[-1])
        return ret

class Container(object):
    """
    Represents a type of child block found in an haproxy config.

    Intended to be subclassed by various types of child blocks, like
    Locations or Geo blocks.
    """

    def __init__(self, value, *args):
        """
        Initialize object.

        :param str value: Value to be used in name (e.g. regex for Location)
        :param *args: Any objects to include in this Conf.
        """
        self.name = ''
        self.value = value
        self._depth = 0
        self.children = list(args)
        bump_child_depth(self, self._depth)

    def add(self, *args):
        """
        Add object(s) to the Container.

        :param *args: Any objects to add to the Container.
        :returns: full list of Container's child objects
        """
        self.children.extend(args)
        bump_child_depth(self, self._depth)
        return self.children

    def remove(self, *args):
        """
        Remove object(s) from the Container.

        :param *args: Any objects to remove from the Container.
        :returns: full list of Container's child objects
        """
        for x in args:
            self.children.remove(x)
        return self.children

    def filter(self, btype='', name=''):
        """
        Return child object(s) of this Server block that meet certain criteria.

        :param str btype: Type of object to filter by (e.g. 'Key')
        :param str name: Name of key OR container value to filter by
        :returns: full list of matching child objects
        """
        filtered = []
        for x in self.children:
            if name and isinstance(x, Key) and x.name == name:
                filtered.append(x)
            elif isinstance(x, Container) and x.__class__.__name__ == btype\
                    and x.value == name:
                filtered.append(x)
            elif not name and btype and x.__class__.__name__ == btype:
                filtered.append(x)
        return filtered

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return new_dict

    @property
    def as_strings(self):
        """Return the entire Container as haproxy config strings."""
        ret = []
        container_title = (INDENT * self._depth)
        container_title += '{0}{1} \n'.format(
            self.name, (' {0}'.format(self.value) if self.value else '')
        )
        ret.append(container_title)
        for x in self.children:
            if isinstance(x, Key):
                ret.append(INDENT + x.as_strings)
            elif isinstance(x, Container):
                y = x.as_strings
                ret.append('\n' + y[0])
                for z in y[1:]:
                    ret.append(INDENT + z)
            else:
                y = x.as_strings
                ret.append(INDENT + y)
        ret[-1] = re.sub('}\n+$', '}\n', ret[-1])
        ret.append('\n')
        return ret

class Global(Container):
    """Container for global options."""

    def __init__(self, *args):
        """Initialize."""
        super(Global, self).__init__('', *args)
        self.name = 'global'
    
    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'global': new_dict}

class Defaults(Container):
    """Container for defaults options."""

    def __init__(self, *args):
        """Initialize."""
        super(Defaults, self).__init__("", *args)
        self.name = 'defaults'
    
    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        for x in self.children:
            if x.name not in new_dict.keys():
                new_dict.update(x.as_dict)
            else:
                if isinstance(new_dict[x.name], list):
                    new_dict[x.name].append(x.value)
                else:
                    tmp_list = [new_dict[x.name]]
                    tmp_list.append(x.value)
                    new_dict[x.name] = tmp_list
        return {'defaults': new_dict}

class Frontend(Container):
    """Container for frontend options."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Frontend, self).__init__(value, *args)
        self.name = 'frontend'
        self.value = value
    
    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        if not new_dict.has_key(self.value):
            new_dict[self.value] = {}
        for x in self.children:
            k = x.as_dict.keys()[0]
            v = x.as_dict[k]
            if k not in new_dict[self.value].keys():
                new_dict[self.value].update({k: v})
            else:
                if isinstance(new_dict[self.value][k], list):
                    new_dict[self.value][k].append(v)
                else:
                    tmp_list = [new_dict[self.value][k]]
                    tmp_list.append(v)
                    new_dict[self.value][k] = v
        return {'frontend': new_dict}

class Backend(Container):
    """Container for backend options."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Backend, self).__init__(value, *args)
        self.name = 'backend'
        self.value = value
    
    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        for x in self.children:
            if not new_dict.has_key(self.value):
                new_dict[self.value] = {}
            k = x.as_dict.keys()[0]
            v = x.as_dict[k]
            if k not in new_dict[self.value].keys():
                if k == "server":
                    new_dict[self.value][k] = [v]
                else:
                    new_dict[self.value].update({k: v})
            else:
                if isinstance(new_dict[self.value][k], list):
                    new_dict[self.value][k].append(v)
                else:
                    tmp_list = [new_dict[self.value][k]]
                    tmp_list.append(v)
                    new_dict[self.value][k] = tmp_list
        return {'backend': new_dict}

class Listen(Container):
    """Container for listen options."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Listen, self).__init__(value, *args)
        self.name = 'listen'
        self.value = value
    
    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'listen': new_dict}

class Key(object):
    """Represents a simple key/value object found in an haproxy config."""

    def __init__(self, name, value):
        """
        Initialize object.

        :param *args: Any objects to include in this Server block.
        """
        self.name = name
        self.value = value
        if isinstance(self.value, int):
            raise Exception("Type error, key <{0}>'s value must be str".format(self.name))

    @property
    def as_dict(self):
        """Return key as dict key/value."""
        return {self.name: self.value}

    @property
    def as_strings(self):
        """Return key as haproxy config string."""
        if self.value == '' or self.value is None:
            return '{0}\n'.format(self.name)
        if '"' not in self.value and '#' in self.value:
            return '{0} "{1}"\n'.format(self.name, self.value)
        return '{0} {1}\n'.format(self.name, self.value)

def bump_child_depth(obj, depth):
    children = getattr(obj, 'children', [])
    for child in children:
        child._depth = depth + 1
        bump_child_depth(child, child._depth)

class HAProxyAgent(object):
    @classmethod
    def load_json(cls, data):
        CLASS_NAMES = {'global': Global, 'defaults': Defaults, 'frontend': Frontend, 'backend': Backend, 'listen': Listen}
        haproxy_obj = Conf()
        if isinstance(data, dict):
            for k1, v1 in data['conf'].items():
                if isinstance(v1, dict):
                    if k1 == 'global':
                        global_obj = Global()
                        haproxy_obj.add(global_obj)
                        for k2, v2 in data['conf']['global'].items():
                            if isinstance(v2, list):
                                for v3 in v2:
                                    global_obj.add(Key(k2, v3))
                            else:
                                global_obj.add(Key(k2, v2))
                    elif k1 == 'defaults':
                        defaults_obj = Defaults()
                        haproxy_obj.add(defaults_obj)
                        for k2, v2 in data['conf']['defaults'].items():
                            if isinstance(v2, list):
                                for v3 in v2:
                                    defaults_obj.add(Key(k2, v3))
                            else:
                                defaults_obj.add(Key(k2, v2))
                    elif k1 == 'frontend':
                        for k2, v2 in data['conf']['frontend'].items():
                            frontend_obj = Frontend(k2)
                            haproxy_obj.add(frontend_obj)
                            for k3, v3 in data['conf']['frontend'][k2].items():
                                if isinstance(v3, list):
                                    for v4 in v3:
                                        frontend_obj.add(Key(k3, v4))
                                else:
                                    frontend_obj.add(Key(k3, v3))
                    elif k1 == 'backend':
                        for k2, v2 in data['conf']['backend'].items():
                            backend_obj = Backend(k2)
                            haproxy_obj.add(backend_obj)
                            for k3, v3 in data['conf']['backend'][k2].items():
                                if isinstance(v3, list):
                                    for v4 in v3:
                                        backend_obj.add(Key(k3, v4))
                                else:
                                    backend_obj.add(Key(k3, v3))
                    elif k1 == 'listen':
                        listen_obj = Listen(k2)
                        haproxy_obj.add(listen_obj)
                        for k2, v2 in data['conf']['listen'].items():
                            listen_obj.add(Key(k2, v2))
                    else:
                        raise Exception("Input error, cannot handle key {0}, please connect vendor".format(k1))
                else:
                    raise Exception("Input error, key {0}'s value must be json".format(k1))
        else:
            raise Exception("Input error, need json data")

        return haproxy_obj

    @classmethod
    def load_conf(cls, data, conf=True):
        """
        Load an haproxy configuration from a provided string.

        :param str data: haproxy configuration
        :param bool conf: Load object(s) into a Conf object?
        """
        f = Conf() if conf else []
        lopen = []
        index = 0

        while True:
            m = re.compile(r'^global\s*', re.S).search(data[index:])
            if m:
                glob = Global()
                lopen.insert(0, glob)
                index += m.end()
                continue

            m = re.compile(r'^defaults\s*', re.S).search(data[index:])
            if m:
                defaults = Defaults()
                lopen.insert(0, defaults)
                index += m.end()
                continue

            m = re.compile(r'^frontend\s*(.*?)\n', re.S).search(data[index:])
            if m:
                frontend = Frontend(m.group(1).strip())
                lopen.insert(0, frontend)
                index += m.end()
                continue

            m = re.compile(r'^backend\s*(.*?)\n', re.S).search(data[index:])
            if m:
                backend = Backend(m.group(1).strip())
                lopen.insert(0, backend)
                index += m.end()
                continue

            m = re.compile(r'^listen\s*', re.S).search(data[index:])
            if m:
                listen = Listen()
                lopen.insert(0, listen)
                index += m.end()
                continue

            m = re.compile(r'^\s*\n', re.S).search(data[index:])
            if m:
                index += m.end()
                continue

            m = re.compile(r'^(\s*)#[ \r\t\f]*(.*?)\n', re.S).search(data[index:])
            if m:
                index += m.end()
                continue

            m = re.compile(r'^\s*(.+)\n').search(data[index:])
            if m:
                k_list = m.group(1).split(" ")
                if len(k_list) == 1:
                    k = Key(k_list[0].strip(), "")
                elif len(k_list) == 2:
                    k = Key(k_list[0].strip(), k_list[1])
                else:
                    k = Key(k_list[0].strip(), " ".join(k_list[1:]))

                if lopen and isinstance(lopen[0], (Container, Global, Defaults, Frontend, Backend, Listen)):
                    lopen[0].add(k)
                else:
                    f.add(k) if conf else f.append(k)
                index += m.end()
                continue

            break
       
        for obj in lopen:
            f.add(obj) if conf else f.append(obj)
        return f

    @classmethod
    def loadf(cls, path):
        """
        Load an haproxy configuration from a provided file path.

        :param file path: path to haproxy configuration on disk
        """
        with open(path, 'r') as f:
            return cls.load_conf(f.read())
    
    @classmethod
    def dumps(cls, obj):
        """
        Dump an haproxy configuration to a string.

        :param obj obj: haproxy object (Conf, Server, Container)
        :returns: haproxy configuration as string
        """
        return ''.join(obj.as_strings)

    @classmethod
    def dumpf(cls, obj, path):
        """
        Write an haproxy configuration to file.

        :param obj obj: haproxy object (Conf, Container ...)
        :param str path: path to haproxy configuration on disk
        :returns: path the configuration was written to
        """
        # syslog(MODULE_NAME, "Writing haproxy config {0}".format(path))
        # 按顺序写入文件，global > defaults > frontend > backend > listen
        new_obj = Conf()
        global_obj = obj.filter(btype='Global')
        if global_obj:
            new_obj.add(global_obj[0])
        defaults_obj = obj.filter(btype='Defaults')
        if defaults_obj:
            for k in defaults_obj:
                new_obj.add(k)
        frontend_obj = obj.filter(btype='Frontend')
        if frontend_obj:
            for k in frontend_obj:
                new_obj.add(k)
        backend_obj = obj.filter(btype='Backend')
        if backend_obj:
            for k in backend_obj:
                new_obj.add(k) 
        listen_obj = obj.filter(btype='Listen')
        if listen_obj:
            for k in listen_obj:
                new_obj.add(k) 

        with open(path, 'w') as f:
            f.write(cls.dumps(new_obj))
        return path
    
    @classmethod
    def reset_failed_haproxy_service(cls):
        """
        systemctl restart haproxy
        """
        cmd = "systemctl reset-failed %s" % SERVICE_NAME
        (outmsg, errmsg) = cmdprocess.shell_cmd(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg)

    @classmethod
    def restart_haproxy_service(cls):
        """start haproxy service"""
        cmd = "systemctl reload-or-restart %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)
    
    @classmethod
    def force_restart_haproxy_service(cls):
        """start haproxy service"""
        cmd = "systemctl restart %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)

    @classmethod
    def start_haproxy_service(cls):
        """start haproxy service"""
        cmd = "systemctl start %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)

    @classmethod
    def stop_haproxy_service(cls):
        """stop haproxy service"""
        cmd = "systemctl stop %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)
    
    @classmethod
    def enable_haproxy_service(cls):
        """enable haproxy service"""
        cmd = "systemctl enable %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)
    
    @classmethod
    def disable_haproxy_service(cls):
        """disable haproxy service"""
        cmd = "systemctl disable %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)
    
    @classmethod
    def get_haproxy_status(cls):
        """haproxy service status"""
        cmd = "systemctl status %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        if returncode == 0:
            return True
        else:
            return False

    @classmethod
    def get_haproxy_enabled(cls):
        """haproxy is-enabled status"""
        cmd = "systemctl is-enabled %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        if returncode == 0:
            return True
        else:
            return False

    @classmethod
    def is_haproxy_conf_ready(cls):
        """get haproxy config"""
        if os.path.exists(HAPROXY_CONFIG):
            conf = cls.loadf(HAPROXY_CONFIG)
            if len(conf.as_strings) == 0:
                return False
            else:
                return True
        else:
            return False

    @classmethod
    def is_haproxy_process_ready(cls):
        """check haproxy process is ready"""
        conf = cls.loadf(HAPROXY_CONFIG)
        if len(conf.filter(btype='Backend', name="cs")) > 0:
            port = re.search(r'\d+', conf.filter(btype='Frontend', name="cs")[0].as_dict['frontend']['cs']['bind']).group()
            cmd = "curl --max-time 3 -v -k https://proton-cs.lb.aishu.cn:"+port+"/healthz"
            (returncode, errmsg, outmsg) = cmdprocess.shell_cmd_not_raise(cmd, timeout_seconds=4)

            api_cmd = "curl --max-time 3 -v -k https://127.0.0.1:6443/healthz"
            (api_returncode, api_errmsg, api_outmsg) = cmdprocess.shell_cmd_not_raise(api_cmd, timeout_seconds=4)

            #如果apiserver能连但haproxy不能连才重启haproxy，防止初始化时haproxy被不断重启
            if returncode != 0 and api_returncode == 0:
                syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)
                syslog_cmd(MODULE_NAME, api_cmd, api_outmsg, api_errmsg, api_returncode)
                return False
        return True

    @classmethod
    def test_haproxy_conf(cls):
        cmd = "/usr/local/haproxy/sbin/haproxy -c -f {0}".format(HAPROXY_CONFIG)
        (returncode, errmsg, outmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        if returncode == 0:
            return None, returncode
        else:
            return outmsg.strip(), returncode

    @classmethod
    def clear_haproxy(cls):
        """清理haproxy"""
        syslog(MODULE_NAME, 'Clear haproxy.cfg begin')

        # 清理配置文件
        if os.path.exists(HAPROXY_CONFIG):
            filelib.write_file(HAPROXY_CONFIG, '', 'w+')

        syslog(MODULE_NAME, 'Clear haproxy.cfg end')
    
    
