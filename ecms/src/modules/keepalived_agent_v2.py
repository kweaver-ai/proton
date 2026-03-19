#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""
Python library for editing keepalived serverblocks.
"""

import re
import os
import time
from src.modules.pydeps import cmdprocess, filelib
from src.modules.pydeps.logger import syslog, syslog_cmd

INDENT = '    '

SERVICE_NAME = 'keepalived'
KEEPALIVED_CONFIG = "/etc/keepalived/keepalived.conf"
SYSCONF_DESC = '/etc/sysconfig/keepalived'
MODULE_NAME = 'KeepalivedAgent'


class Conf(object):
    """
    Represents an keepalived configuration.

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
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            if d.has_key('vrrp_instance') and new_dict.has_key('vrrp_instance'):
                new_dict['vrrp_instance'].update(d['vrrp_instance'])
            elif d.has_key('virtual_server') and new_dict.has_key('virtual_server'):
                new_dict['virtual_server'].update(d['virtual_server'])
            elif d.has_key('vrrp_script') and new_dict.has_key('vrrp_script'):
                new_dict['vrrp_script'].update(d['vrrp_script'])
            else:
                new_dict.update(d)

        return {'conf': new_dict}

    @property
    def as_strings(self):
        """Return the entire Conf as keepalived config strings."""
        ret = []
        for x in self.children:
            if isinstance(x, (Key, Comment)):
                ret.append(x.as_strings)
            else:
                for y in x.as_strings:
                    ret.append(y)
        if ret:
            ret[-1] = re.sub('}\n+$', '}\n', ret[-1])
        return ret


class Container(object):
    """
    Represents a type of child block found in an keepalived config.

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
        """Return the entire Container as keepalived config strings."""
        ret = []
        container_title = (INDENT * self._depth)
        container_title += '{0}{1} {{\n'.format(
            self.name, (' {0}'.format(self.value) if self.value else '')
        )
        ret.append(container_title)
        for x in self.children:
            if isinstance(x, Key):
                ret.append(INDENT + x.as_strings)
            elif isinstance(x, Comment):
                if x.inline and len(ret) >= 1:
                    ret[-1] = ret[-1].rstrip('\n') + '  ' + x.as_stringsas_strings
                else:
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
        ret.append('}\n\n')
        return ret


class Comment(object):
    """Represents a comment in an keepalived config."""

    def __init__(self, comment, inline=False):
        """
        Initialize object.

        :param str comment: Value of the comment
        :param bool inline: This comment is on the same line as preceding item
        """
        self.comment = comment
        self.inline = inline

    @property
    def as_dict(self):
        """Return comment as dict."""
        if self.comment:
            return {'#': self.comment}

    @property
    def as_strings(self):
        """Return comment as keepalived config string."""
        return '# {0}\n'.format(self.comment)


class Global_defs(Container):
    """Container for global_defs options."""

    def __init__(self, *args):
        """Initialize."""
        super(Global_defs, self).__init__('', *args)
        self.name = 'global_defs'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'global_defs': new_dict}


class Vrrp_instance(Container):
    """Container for vrrp_instance options."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Vrrp_instance, self).__init__(value, *args)
        self.name = 'vrrp_instance'
        self.value = value

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        if len(self.children) == 0:
            new_dict[self.value] = {}
        else:
            for x in self.children:
                if not new_dict.has_key(self.value):
                    new_dict[self.value] = {}
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
                        new_dict[self.value][k] = tmp_list

        return {'vrrp_instance': new_dict}


class Authentication(Container):
    """Container for authentication options."""

    def __init__(self, *args):
        """Initialize."""
        super(Authentication, self).__init__('', *args)
        self.name = 'authentication'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        # return {'server': [x.as_dict for x in self.children]}
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'authentication': new_dict}


class Virtual_ipaddress(Container):
    """Container for virtual_ipaddress options."""

    def __init__(self, *args):
        """Initialize."""
        super(Virtual_ipaddress, self).__init__('', *args)
        self.name = 'virtual_ipaddress'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        # return {'server': [x.as_dict for x in self.children]}
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'virtual_ipaddress': new_dict}


class Track_script(Container):
    """Container for track_script options."""

    def __init__(self, *args):
        """Initialize."""
        super(Track_script, self).__init__('', *args)
        self.name = 'track_script'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        # return {'server': [x.as_dict for x in self.children]}
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'track_script': new_dict}


class Unicast_peer(Container):
    """Container for unicast_peer options."""

    def __init__(self, *args):
        """Initialize."""
        super(Unicast_peer, self).__init__('', *args)
        self.name = 'unicast_peer'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        # return {'server': [x.as_dict for x in self.children]}
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'unicast_peer': new_dict}


class Vrrp_script(Container):
    """Container for vrrp_script options."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Vrrp_script, self).__init__(value, *args)
        self.name = 'vrrp_script'
        self.value = value

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        if len(self.children) == 0:
            new_dict[self.value] = {}
        else:
            for x in self.children:
                if not new_dict.has_key(self.value):
                    new_dict[self.value] = {}
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
                        new_dict[self.value][k] = tmp_list

        return {'vrrp_script': new_dict}


class Virtual_server(Container):
    """Container for virtual_server options."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Virtual_server, self).__init__(value, *args)
        self.name = 'virtual_server'
        self.value = value

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        if len(self.children) == 0:
            new_dict[self.value] = {}
        else:
            for x in self.children:
                if not new_dict.has_key(self.value):
                    new_dict[self.value] = {}
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
                        new_dict[self.value][k] = tmp_list

        return {'virtual_server': new_dict}


class Real_server(Container):
    """Container for real_server options."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Real_server, self).__init__(value, *args)
        self.name = 'real_server'
        self.value = value

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        if len(self.children) == 0:
            new_dict[self.value] = {}
        else:
            for x in self.children:
                if not new_dict.has_key(self.value):
                    new_dict[self.value] = {}
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
                        new_dict[self.value][k] = tmp_list

        return {'real_server': new_dict}


class Tcp_check(Container):
    """Container for TCP_CHECK options."""

    def __init__(self, *args):
        """Initialize."""
        super(Tcp_check, self).__init__('', *args)
        self.name = 'TCP_CHECK'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'TCP_CHECK': new_dict}


class Misc_check(Container):
    """Container for MISC_CHECK options."""

    def __init__(self, *args):
        """Initialize."""
        super(Misc_check, self).__init__('', *args)
        self.name = 'MISC_CHECK'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'MISC_CHECK': new_dict}


class Key(object):
    """Represents a simple key/value object found in an keepalived config."""

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
        """Return key as keepalived config string."""
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


class KeepalivedAgent(object):
    @classmethod
    def load_json(cls, data):
        CLASS_NAMES = {'global_defs': Global_defs, 'vrrp_instance': Vrrp_instance, 'virtual_ipaddress': Virtual_ipaddress,
                       'track_script': Track_script, 'unicast_peer': Unicast_peer, 'vrrp_script': Vrrp_script,
                       'virtual_server': Virtual_server, 'real_server': Real_server, 'TCP_CHECK': Tcp_check, 'MISC_CHECK': Misc_check}
        keepalived_obj = Conf()
        if isinstance(data, dict):
            for k1, v1 in data['conf'].items():
                if isinstance(v1, dict):
                    if k1 == 'vrrp_instance':
                        for k2, v2 in data['conf']['vrrp_instance'].items():
                            vrrp_inst_obj = Vrrp_instance(k2)
                            keepalived_obj.add(vrrp_inst_obj)
                            if isinstance(v2, dict):
                                for k3, v3 in v2.items():
                                    if k3 not in CLASS_NAMES.keys():
                                        vrrp_inst_obj.add(Key(k3, v3))
                                    else:
                                        tmp_obj = CLASS_NAMES[k3]()
                                        if isinstance(v3, dict):
                                            for k4, v4 in v3.items():
                                                tmp_obj.add(Key(k4, v4))
                                        elif isinstance(v3, list):
                                            for k in v3:
                                                tmp_obj.add(Key(k, ""))
                                        vrrp_inst_obj.add(tmp_obj)
                    elif k1 == 'vrrp_script':
                        for k2, v2 in data['conf']['vrrp_script'].items():
                            vrrp_script_obj = Vrrp_script(k2)
                            keepalived_obj.add(vrrp_script_obj)
                            for k3, v3 in v2.items():
                                vrrp_script_obj.add(Key(k3, v3))
                    elif k1 == 'global_defs':
                        global_defs_obj = Global_defs()
                        keepalived_obj.add(global_defs_obj)
                        for k2, v2 in data['conf']['global_defs'].items():
                            global_defs_obj.add(Key(k2, v2))
                    elif k1 == 'virtual_server':
                        for k2, v2 in data['conf']['virtual_server'].items():
                            virtual_obj = Virtual_server(k2)
                            keepalived_obj.add(virtual_obj)
                            for k3, v3 in v2.items():
                                if k3 != 'real_server':
                                    virtual_obj.add(Key(k3, v3))
                                else:
                                    if isinstance(v3, list):
                                        for rs in v3:
                                            for k4, v4 in rs.items():
                                                rs_obj = Real_server(k4)
                                                virtual_obj.add(rs_obj)
                                                if isinstance(v4, dict):
                                                    for k5, v5 in v4.items():
                                                        if k5 in CLASS_NAMES.keys():
                                                            tmp_obj = CLASS_NAMES[k5]()
                                                            rs_obj.add(tmp_obj)
                                                            for k6, v6 in v5.items():
                                                                tmp_obj.add(Key(k6, v6))
                                                        else:
                                                            rs_obj.add(Key(k5, v5))
                                                else:
                                                    raise Exception(
                                                        "Input error, key {0}'s value must be json".format(k1))
                                    elif isinstance(v3, dict):
                                        for k4, v4 in v3.items():
                                            rs_obj = Real_server(k4)
                                            virtual_obj.add(rs_obj)
                                            if isinstance(v4, dict):
                                                for k5, v5 in v4.items():
                                                    if k5 in CLASS_NAMES.keys():
                                                        tmp_obj = CLASS_NAMES[k5]()
                                                        rs_obj.add(tmp_obj)
                                                        for k6, v6 in v5.items():
                                                            tmp_obj.add(Key(k6, v6))
                                                    else:
                                                        rs_obj.add(Key(k5, v5))
                                            else:
                                                raise Exception("Input error, key {0}'s value must be json".format(k1))
                    else:
                        raise Exception("Input error, cannot handle key {0}, please connect vendor".format(k1))
                else:
                    raise Exception("Input error, key {0}'s value must be json".format(k1))
        else:
            raise Exception("Input error, need json data")

        return keepalived_obj

    @classmethod
    def load_conf(cls, data, conf=True):
        """
        Load an keepalived configuration from a provided string.

        :param str data: keepalived configuration
        :param bool conf: Load object(s) into a Conf object?
        """
        f = Conf() if conf else []
        lopen = []
        index = 0

        while True:
            m = re.compile(r'^\s*global_defs\s*{', re.S).search(data[index:])
            if m:
                glob = Global_defs()
                lopen.insert(0, glob)
                index += m.end()
                continue

            m = re.compile(r'^\s*vrrp_instance\s*([^;]*?)\s*{', re.S).search(data[index:])
            if m:
                ins = Vrrp_instance(m.group(1))
                lopen.insert(0, ins)
                index += m.end()
                continue

            m = re.compile(r'^\s*authentication\s*{', re.S).search(data[index:])
            if m:
                auth = Authentication()
                lopen.insert(0, auth)
                index += m.end()
                continue

            m = re.compile(r'^\s*virtual_ipaddress\s*{', re.S).search(data[index:])
            if m:
                vip = Virtual_ipaddress()
                lopen.insert(0, vip)
                index += m.end()
                continue

            m = re.compile(r'^\s*unicast_peer\s*{', re.S).search(data[index:])
            if m:
                unic = Unicast_peer()
                lopen.insert(0, unic)
                index += m.end()
                continue

            m = re.compile(r'^\s*track_script\s*{', re.S).search(data[index:])
            if m:
                trac = Track_script()
                lopen.insert(0, trac)
                index += m.end()
                continue

            m = re.compile(r'^\s*vrrp_script\s*([^;]*?)\s*{', re.S).search(data[index:])
            if m:
                vrsc = Vrrp_script(m.group(1))
                lopen.insert(0, vrsc)
                index += m.end()
                continue

            m = re.compile(r'^\s*virtual_server\s*([^;]*?)\s*{', re.S).search(data[index:])
            if m:
                vs = Virtual_server(m.group(1))
                lopen.insert(0, vs)
                index += m.end()
                continue

            m = re.compile(r'^\s*real_server\s*([^;]*?)\s*{', re.S).search(data[index:])
            if m:
                rs = Real_server(m.group(1))
                lopen.insert(0, rs)
                index += m.end()
                continue

            m = re.compile(r'^\s*TCP_CHECK\s*{', re.S).search(data[index:])
            if m:
                tck = Tcp_check()
                lopen.insert(0, tck)
                index += m.end()
                continue

            m = re.compile(r'^\s*MISC_CHECK\s*{', re.S).search(data[index:])
            if m:
                mck = Misc_check()
                lopen.insert(0, mck)
                index += m.end()
                continue

            m = re.compile(r'^\s*}', re.S).search(data[index:])
            if m:
                if isinstance(lopen[0], Container):
                    c = lopen[0]
                    lopen.pop(0)
                    if lopen and isinstance(lopen[0], Container):
                        lopen[0].add(c)
                    else:
                        f.add(c) if conf else f.append(c)
                index += m.end()
                continue

            m = re.compile(r'^\s*(.+)\n').search(data[index:])
            if m:
                k_list = m.group(1).split(" ")
                if len(k_list) == 1:
                    k = Key(k_list[0], "")
                elif len(k_list) == 2:
                    k = Key(k_list[0], k_list[1])
                else:
                    k = Key(k_list[0], " ".join(k_list[1:]))

                if lopen and isinstance(lopen[0], (Container, Global_defs, Vrrp_instance, Vrrp_script, Virtual_server, Real_server)):
                    lopen[0].add(k)
                else:
                    f.add(k) if conf else f.append(k)
                index += m.end()
                continue

            break

        return f

    @classmethod
    def loadf(cls, path):
        """
        Load an keepalived configuration from a provided file path.

        :param file path: path to keepalived configuration on disk
        """
        with open(path, 'r') as f:
            return cls.load_conf(f.read())

    @classmethod
    def dumps(cls, obj):
        """
        Dump an keepalived configuration to a string.

        :param obj obj: keepalived object (Conf, Server, Container)
        :returns: keepalived configuration as string
        """
        return ''.join(obj.as_strings)

    @classmethod
    def dumpf(cls, obj, path):
        """
        Write an keepalived configuration to file.

        :param obj obj: keepalived object (Conf, Container ...)
        :param str path: path to keepalived configuration on disk
        :returns: path the configuration was written to
        """
        # syslog(MODULE_NAME, "Writing keepalived config {0}".format(path))
        # 按顺序写入文件，global_defs > vrrp_script > vrrp_instance > virtual_server
        new_obj = Conf()
        global_obj = obj.filter(btype='Global_defs')
        if global_obj:
            new_obj.add(global_obj[0])
        vrrp_script_obj = obj.filter(btype='Vrrp_script')
        if vrrp_script_obj:
            for k in vrrp_script_obj:
                new_obj.add(k)
        instance_obj = obj.filter(btype='Vrrp_instance')
        if instance_obj:
            for k in instance_obj:
                new_obj.add(k)
        vs_obj = obj.filter(btype='Virtual_server')
        if vs_obj:
            for k in vs_obj:
                new_obj.add(k)

        with open(path, 'w') as f:
            f.write(cls.dumps(new_obj))
        return path

    @classmethod
    def reset_failed_keepalived_service(cls):
        """
        systemctl restart keepalived
        """
        cmd = "systemctl reset-failed %s" % SERVICE_NAME
        (outmsg, errmsg) = cmdprocess.shell_cmd(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg)

    @classmethod
    def restart_keepalived_service(cls):
        """start keepalived service"""
        cmd = "systemctl restart %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)

    @classmethod
    def start_keepalived_service(cls):
        """start keepalived service"""
        cmd = "systemctl start %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)

    @classmethod
    def stop_keepalived_service(cls):
        """stop keepalived service"""
        cmd = "systemctl stop %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)

    @classmethod
    def enable_keepalived_service(cls):
        """enable keepalived service"""
        cmd = "systemctl enable %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)

    @classmethod
    def disable_keepalived_service(cls):
        """disable keepalived service"""
        cmd = "systemctl disable %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)

    @classmethod
    def is_failed_keepalived_service(cls):
        """Check whether keepalived.service is in "failed" state"""
        cmd = "systemctl is-failed %s" % SERVICE_NAME
        rc, out, err = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, out, err, rc)
        return rc == 0

    @classmethod
    def get_keepalived_status(cls):
        """keepalived service status"""
        cmd = "systemctl status %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        if returncode == 0:
            return True
        else:
            return False

    @classmethod
    def get_keepalived_enabled(cls):
        """keepalived is-enabled status"""
        cmd = "systemctl is-enabled %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        if returncode == 0:
            return True
        else:
            return False

    @classmethod
    def reload_keepalived_service(cls, is_vrrp_changed=False):
        """reload keepalived service"""
        cmd = "systemctl reload %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg, returncode)

        # 等待15s,确保脚本执行完成
        if is_vrrp_changed is True:
            syslog(MODULE_NAME, "Wait 15s for keepalived transition begin.")
            time.sleep(15)
            syslog(MODULE_NAME, "Wait 15s for keepalived transition end.")
        else:
            syslog(MODULE_NAME, "Wait 3s for keepalived reload begin.")
            time.sleep(3)
            syslog(MODULE_NAME, "Wait 3s for keepalived reload end.")

    @classmethod
    def get_version(cls):
        cmd = "keepalived -v"
        (returncode, errmsg, outmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        if returncode != 127:
            current_keepalived_version = outmsg.split("\n")[0].split(" ")[1]
        else:
            current_keepalived_version = ''
        return current_keepalived_version

    @classmethod
    def test_keepalived_conf(cls):
        cmd = "keepalived -t"
        (returncode, errmsg, outmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        if outmsg.strip() == "SECURITY VIOLATION - scripts are being executed but script_security not enabled." or returncode == 0:
            return None
        else:
            return outmsg.strip()

    @classmethod
    def clear_keepalived(cls):
        """清理keepalived"""
        syslog(MODULE_NAME, 'Clear keepalived.conf begin')

        # 清理配置文件
        if os.path.exists(KEEPALIVED_CONFIG):
            filelib.write_file(KEEPALIVED_CONFIG, '', 'w+')

        syslog(MODULE_NAME, 'Clear keepalived.conf end')
