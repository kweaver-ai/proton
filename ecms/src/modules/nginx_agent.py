#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""
Python library for editing NGINX serverblocks.
"""

import re
import os
from src.modules.pydeps import cmdprocess
from src.modules.pydeps.logger import syslog, syslog_cmd

INDENT = '    '

SERVICE_NAME = "slb-nginx"
MODULE_NAME = "NginxAgent"
SLB_NGINX_SBIN = "/usr/local/slb-nginx/sbin/slb-nginx"
NGINX_INCLUDE_DIR = '/usr/local/slb-nginx/conf.d'
HTTP_INCLUDE_DIR = os.path.join(NGINX_INCLUDE_DIR, 'http')
STREAM_INCLUDE_DIR = os.path.join(NGINX_INCLUDE_DIR, 'stream')


class Conf(object):
    """
    Represents an nginx configuration.

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
            elif isinstance(x, Container) and x.__class__.__name__ == btype \
                    and x.value == name:
                filtered.append(x)
            elif not name and btype and x.__class__.__name__ == btype:
                filtered.append(x)
        return filtered

    @property
    def servers(self):
        """Return a list of child Server objects."""
        return [x for x in self.children if isinstance(x, Server)]

    @property
    def server(self):
        """Convenience property to fetch the first available server only."""
        return self.servers[0]

    @property
    def as_list(self):
        """Return all child objects in nested lists of strings."""
        return [x.as_list for x in self.children]

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        # return {'conf': [x.as_dict for x in self.children]}
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            if d.has_key('upstream') and new_dict.has_key('upstream'):
                new_dict['upstream'].update(d['upstream'])
            else:
                new_dict.update(d)

        return {'conf': new_dict}

    @property
    def as_strings(self):
        """Return the entire Conf as nginx config strings."""
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
    Represents a type of child block found in an nginx config.

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
            elif isinstance(x, Container) and x.__class__.__name__ == btype \
                    and x.value == name:
                filtered.append(x)
            elif not name and btype and x.__class__.__name__ == btype:
                filtered.append(x)
        return filtered

    @property
    def locations(self):
        """Return a list of child Location objects."""
        return [x for x in self.children if isinstance(x, Location)]

    @property
    def comments(self):
        """Return a list of child Comment objects."""
        return [x for x in self.children if isinstance(x, Comment)]

    @property
    def keys(self):
        """Return a list of child Key objects."""
        return [x for x in self.children if isinstance(x, Key)]

    @property
    def as_list(self):
        """Return all child objects in nested lists of strings."""
        return [self.name, self.value, [x.as_list for x in self.children]]

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
        """Return the entire Container as nginx config strings."""
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
    """Represents a comment in an nginx config."""

    def __init__(self, comment, inline=False):
        """
        Initialize object.

        :param str comment: Value of the comment
        :param bool inline: This comment is on the same line as preceding item
        """
        self.comment = comment
        self.inline = inline

    @property
    def as_list(self):
        """Return comment as nested list of strings."""
        return [self.comment]

    @property
    def as_dict(self):
        """Return comment as dict."""
        if self.comment:
            return {'#': self.comment}

    @property
    def as_strings(self):
        """Return comment as nginx config string."""
        return '# {0}\n'.format(self.comment)


class Http(Container):
    """Container for HTTP sections in the main NGINX conf file."""

    def __init__(self, *args):
        """Initialize."""
        super(Http, self).__init__('', *args)
        self.name = 'http'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        for x in self.children:
            k = x.as_dict.keys()[0]
            v = x.as_dict[k]
            if k not in new_dict.keys():
                new_dict.update({k: v})
            else:
                if isinstance(new_dict[k], list):
                    new_dict[k].append(v)
                else:
                    tmp_list = [new_dict[k]]
                    tmp_list.append(v)
                    new_dict[k] = tmp_list
        return {'http': new_dict}


class Server(Container):
    """Container for server block configurations."""

    def __init__(self, *args):
        """Initialize."""
        super(Server, self).__init__('', *args)
        self.name = 'server'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        for x in self.children:
            if x.as_dict:
                k = x.as_dict.keys()[0]
                v = x.as_dict[k]
                if k not in new_dict.keys():
                    new_dict.update({k: v})
                else:
                    if isinstance(new_dict[k], list):
                        new_dict[k].append(v)
                    else:
                        tmp_list = [new_dict[k]]
                        tmp_list.append(v)
                        new_dict[k] = tmp_list
        return {'server': new_dict}


class Location(Container):
    """Container for Location-based options."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Location, self).__init__(value, *args)
        self.name = 'location'
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

        return {'location': new_dict}


class Events(Container):
    """Container for Event-based options."""

    def __init__(self, *args):
        """Initialize."""
        super(Events, self).__init__('', *args)
        self.name = 'events'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        # return {'server': [x.as_dict for x in self.children]}
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'events': new_dict}


class Limit_except(Container):
    """Container for specifying HTTP method restrictions."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Limit_except, self).__init__(value, *args)
        self.name = 'limit_except'


class Types(Container):
    """Container for MIME type mapping."""

    def __init__(self, *args):
        """Initialize."""
        super(Types, self).__init__('', *args)
        self.name = 'types'


class If(Container):
    """Container for If conditionals."""

    def __init__(self, value, *args):
        """Initialize."""
        super(If, self).__init__(value, *args)
        self.name = 'if'
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
                new_dict[self.value].update({k: v})
            else:
                if isinstance(new_dict[self.value][k], list):
                    new_dict[self.value][k].append(v)
                else:
                    tmp_list = [new_dict[self.value][k]]
                    tmp_list.append(v)
                    new_dict[self.value][k] = tmp_list

        return {'if': new_dict}


class Upstream(Container):
    """Container for upstream configuration (reverse proxy)."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Upstream, self).__init__(value, *args)
        self.name = 'upstream'
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
                new_dict[self.value].update({k: v})
            else:
                if isinstance(new_dict[self.value][k], list):
                    new_dict[self.value][k].append(v)
                else:
                    tmp_list = [new_dict[self.value][k]]
                    tmp_list.append(v)
                    new_dict[self.value][k] = tmp_list
        return {'upstream': new_dict}


class Geo(Container):
    """
    Container for geo module configuration.

    See docs here: http://nginx.org/en/docs/http/ngx_http_geo_module.html
    """

    def __init__(self, value, *args):
        """Initialize."""
        super(Geo, self).__init__(value, *args)
        self.name = 'geo'


class Map(Container):
    """Container for map configuration."""

    def __init__(self, value, *args):
        """Initialize."""
        super(Map, self).__init__(value, *args)
        self.name = 'map'


class Stream(Container):
    """Container for stream sections in the main NGINX conf file."""

    def __init__(self, *args):
        """Initialize."""
        super(Stream, self).__init__('', *args)
        self.name = 'stream'

    @property
    def as_dict(self):
        """Return all child objects in nested dict."""
        new_dict = {}
        dicts = [x.as_dict for x in self.children]
        for d in dicts:
            new_dict.update(d)
        return {'stream': new_dict}


class Key(object):
    """Represents a simple key/value object found in an nginx config."""

    def __init__(self, name, value):
        """
        Initialize object.

        :param *args: Any objects to include in this Server block.
        """
        self.name = name
        self.value = value

    @property
    def as_list(self):
        """Return key as nested list of strings."""
        return [self.name, self.value]

    @property
    def as_dict(self):
        """Return key as dict key/value."""
        return {self.name: self.value}

    @property
    def as_strings(self):
        """Return key as nginx config string."""
        if self.value == '' or self.value is None:
            return '{0};\n'.format(self.name)
        if '"' not in self.value and (';' in self.value or '#' in self.value):
            return '{0} "{1}";\n'.format(self.name, self.value)
        return '{0} {1};\n'.format(self.name, self.value)


def bump_child_depth(obj, depth):
    children = getattr(obj, 'children', [])
    for child in children:
        child._depth = depth + 1
        bump_child_depth(child, child._depth)


class NginxAgent(object):
    """
    Nginx Agent
    """

    @classmethod
    def load_json(cls, data):
        """
        load a json data from a provided json and return nginx object

        :param json data
        """
        CLASS_NAMES = {'conf': Conf, 'events': Events, 'http': Http, 'server': Server, 'stream': Stream,
                       'upstream': Upstream, \
                       'location': Location, 'if': If, 'geo': Geo, 'map': Map, 'limit_except': Limit_except,
                       'types': Types}
        nginx_obj = Conf()
        if isinstance(data, dict):
            for k1, v1 in data['conf'].items():
                if k1 not in CLASS_NAMES.keys():
                    nginx_obj.add(Key(k1, v1))
                elif isinstance(v1, dict):
                    for k2, v2 in v1.items():
                        if k2 not in CLASS_NAMES.keys():
                            if isinstance(v2, str):
                                filter_obj = nginx_obj.filter(btype=k1.capitalize())
                                if filter_obj:
                                    filter_obj[0].add((Key(k2, v2)))
                                else:
                                    tmp_obj = CLASS_NAMES[k1]()
                                    tmp_obj.add(Key(k2, v2))
                                    nginx_obj.add(tmp_obj)

                            elif isinstance(v2, list):
                                for x in v2:
                                    filter_obj = nginx_obj.filter(btype=k1.capitalize())
                                    if filter_obj:
                                        filter_obj[0].add((Key(k2, x)))
                                    else:
                                        tmp_obj = CLASS_NAMES[k1]()
                                        tmp_obj.add(Key(k2, x))
                                        nginx_obj.add(tmp_obj)

                            elif isinstance(v2, dict):
                                for k3, v3 in v2.items():
                                    if k3 not in CLASS_NAMES.keys() or k3 == 'server':
                                        if isinstance(v3, str):
                                            filter_obj = nginx_obj.filter(btype=k1.capitalize(), name=k2)
                                            if filter_obj:
                                                filter_obj[0].add(Key(k3, v3))
                                            else:
                                                tmp_obj = CLASS_NAMES[k1](k2)
                                                tmp_obj.add(Key(k3, v3))
                                                nginx_obj.add(tmp_obj)
                                        if isinstance(v3, list):
                                            for x in v3:
                                                filter_obj = nginx_obj.filter(btype=k1.capitalize(), name=k2)
                                                if filter_obj:
                                                    filter_obj[0].add(Key(k3, x))
                                                else:
                                                    tmp_obj = CLASS_NAMES[k1](k2)
                                                    tmp_obj.add(Key(k3, x))
                                                    nginx_obj.add(tmp_obj)
                        else:
                            if isinstance(v2, str):
                                filter_obj = nginx_obj.filter(btype=k1.capitalize())
                                if filter_obj:
                                    filter_obj[0].add((Key(k2, v2)))
                                else:
                                    tmp_obj = CLASS_NAMES[k1]()
                                    tmp_obj.add(Key(k2, v2))
                                    nginx_obj.add(tmp_obj)

                            elif isinstance(v2, list):
                                for x in v2:
                                    if isinstance(x, dict):
                                        for k3, v3 in x.items():
                                            if not v3:
                                                filter_obj = nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                    btype=k2.capitalize(), name=k3)
                                                if not filter_obj:
                                                    v2_obj = CLASS_NAMES[k2](k3)
                                                    nginx_obj.filter(btype=k1.capitalize())[0].add(v2_obj)
                                            if isinstance(v3, dict):
                                                for k4, v4 in v3.items():
                                                    if k4 not in CLASS_NAMES.keys():
                                                        if isinstance(v4, str):
                                                            filter_obj = nginx_obj.filter(btype=k1.capitalize())
                                                            if not filter_obj:
                                                                k1_obj = CLASS_NAMES[k1]()
                                                                nginx_obj.add(k1_obj)
                                                            filter_obj = nginx_obj.filter(btype=k1.capitalize())[
                                                                0].filter(btype=k2.capitalize(), name=k3)
                                                            if filter_obj:
                                                                nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                                    btype=k2.capitalize(), name=k3)[0].add(Key(k4, v4))
                                                            else:
                                                                tmp_obj = CLASS_NAMES[k2](k3)
                                                                tmp_obj.add(Key(k4, v4))
                                                                nginx_obj.filter(btype=k1.capitalize())[0].add(tmp_obj)
                                                        elif isinstance(v4, list):
                                                            for i in v4:
                                                                filter_obj = nginx_obj.filter(btype=k1.capitalize())[
                                                                    0].filter(btype=k2.capitalize(), name=k3)
                                                                if not filter_obj:
                                                                    v2_obj = CLASS_NAMES[k2](k3)
                                                                    v2_obj.add(Key(k4, i))
                                                                    nginx_obj.filter(btype=k1.capitalize())[0].add(
                                                                        v2_obj)
                                                                else:
                                                                    filter_obj[0].add(Key(k4, i))
                                                    else:
                                                        if isinstance(v4, list):
                                                            filter_obj = nginx_obj.filter(btype=k1.capitalize())
                                                            if not filter_obj:
                                                                k1_obj = CLASS_NAMES[k1]()
                                                                nginx_obj.add(k1_obj)
                                                            for x in v4:
                                                                for k5, v5 in x.items():
                                                                    v5_obj = CLASS_NAMES[k4](k5)
                                                                    filter_obj = nginx_obj.filter(btype=k1.capitalize())[
                                                                        0].filter(btype=k2.capitalize(), name=k3)
                                                                    if not filter_obj:
                                                                        v2_obj = CLASS_NAMES[k2](k3)
                                                                        v2_obj.add(v5_obj)
                                                                        nginx_obj.filter(btype=k1.capitalize())[0].add(v2_obj)
                                                                    else:
                                                                        filter_obj[0].add(v5_obj)
                                                                    if v5:
                                                                        if isinstance(v5, dict):
                                                                            for k6, v6 in v5.items():
                                                                                v5_obj.add(Key(k6, v6))
                                                                    else:
                                                                        v5_obj.add(Key('', ''))
                                                        else:
                                                            for k5, v5 in v4.items():
                                                                v5_obj = CLASS_NAMES[k4](k5)
                                                                filter_obj = nginx_obj.filter(btype=k1.capitalize())[
                                                                    0].filter(btype=k2.capitalize(), name=k3)
                                                                if not filter_obj:
                                                                    v2_obj = CLASS_NAMES[k2](k3)
                                                                    v2_obj.add(v5_obj)
                                                                    nginx_obj.filter(btype=k1.capitalize())[0].add(v2_obj)
                                                                else:
                                                                    filter_obj[0].add(v5_obj)
                                                                if v5:
                                                                    if isinstance(v5, dict):
                                                                        for k6, v6 in v5.items():
                                                                            v5_obj.add(Key(k6, v6))
                                                                else:
                                                                    v5_obj.add(Key('', '')) 
 
                            elif isinstance(v2, dict):
                                print v2
                                for k3, v3 in v2.items():
                                    if k3 not in CLASS_NAMES.keys() or k3 == 'server':
                                        if isinstance(v3, str):
                                            filter_obj = nginx_obj.filter(btype=k1.capitalize(), name=k2)
                                            if filter_obj:
                                                filter_obj[0].add(Key(k3, v3))
                                            else:
                                                tmp_obj = CLASS_NAMES[k1](k2)
                                                tmp_obj.add(Key(k3, v3))
                                                nginx_obj.add(tmp_obj)
                                        elif isinstance(v3, list):
                                            for x in v3:
                                                filter_obj = nginx_obj.filter(btype=k1.capitalize(), name=k2)
                                                if filter_obj:
                                                    filter_obj[0].add(Key(k3, x))
                                                else:
                                                    tmp_obj = CLASS_NAMES[k1](k2)
                                                    tmp_obj.add(Key(k3, x))
                                                    nginx_obj.add(tmp_obj)
                                        elif isinstance(v3, dict):
                                            for k4, v4 in v3.items():
                                                if k4 not in CLASS_NAMES.keys() or k4 == 'server':
                                                    if isinstance(v4, str):
                                                        filter_obj = nginx_obj.filter(btype=k1.capitalize())
                                                        if not filter_obj:
                                                            k1_obj = CLASS_NAMES[k1]()
                                                            nginx_obj.add(k1_obj)
                                                        filter_obj = nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                            btype=k2.capitalize(), name=k3)
                                                        if filter_obj:
                                                            nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                                btype=k2.capitalize(), name=k3)[0].add(Key(k4, v4))
                                                        else:
                                                            tmp_obj = CLASS_NAMES[k2](k3)
                                                            tmp_obj.add(Key(k4, v4))
                                                            nginx_obj.filter(btype=k1.capitalize())[0].add(tmp_obj)
                                                        # nginx_obj.add(nginx_obj.filter(btype=k1.capitalize())[0])
                                                    if isinstance(v4, list):
                                                        filter_obj = nginx_obj.filter(btype=k1.capitalize())
                                                        if not filter_obj:
                                                            k1_obj = CLASS_NAMES[k1]()
                                                            nginx_obj.add(k1_obj)
                                                        for x in v4:
                                                            filter_obj = nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                            btype=k2.capitalize(), name=k3)
                                                            if filter_obj:
                                                                nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                                    btype=k2.capitalize(), name=k3)[0].add(Key(k4, x))
                                                            else:
                                                                tmp_obj = CLASS_NAMES[k2](k3)
                                                                tmp_obj.add(Key(k4, x))
                                                                nginx_obj.filter(btype=k1.capitalize())[0].add(tmp_obj)
                                                else:
                                                    # location -> url -> if
                                                    if isinstance(v4, dict):
                                                        for k5, v5 in v4.items():
                                                            if isinstance(v5, dict):
                                                                for k6, v6 in v5.items():
                                                                    if isinstance(v6, str):
                                                                        print v6
                                                                        filter_obj = nginx_obj.filter(btype=k1.capitalize())
                                                                        if not filter_obj:
                                                                            k1_obj = CLASS_NAMES[k1]()
                                                                            nginx_obj.add(k1_obj)
                                                                        filter_obj = nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                                            btype=k2.capitalize(), name=k3)
                                                                        if not filter_obj:
                                                                            tmp_obj = CLASS_NAMES[k2](k3)
                                                                            nginx_obj.filter(btype=k1.capitalize())[0].add(tmp_obj)
                                                                        filter_obj = nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                                            btype=k2.capitalize(), name=k3)[0].filter(btype=k4.capitalize(), name=k5)
                                                                        if filter_obj:
                                                                            filter_obj[0].add(Key(k6, v6))
                                                                        else:
                                                                            tmp_obj = CLASS_NAMES[k4](k5)
                                                                            tmp_obj.add(Key(k6, v6))
                                                                            nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                                            btype=k2.capitalize(), name=k3)[0].add(tmp_obj)
                                                                    # if -> add_header == list value[]
                                                                    if isinstance(v6, list):
                                                                        filter_obj = nginx_obj.filter(btype=k1.capitalize())
                                                                        if not filter_obj:
                                                                            k1_obj = CLASS_NAMES[k1]()
                                                                            nginx_obj.add(k1_obj)
                                                                        filter_obj = nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                                            btype=k2.capitalize(), name=k3)
                                                                        if not filter_obj:
                                                                            tmp_obj = CLASS_NAMES[k2](k3)
                                                                            nginx_obj.filter(btype=k1.capitalize())[0].add(tmp_obj)
                                                                        for x in v6:
                                                                            filter_obj = nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                                                btype=k2.capitalize(), name=k3)[0].filter(btype=k4.capitalize(), name=k5)
                                                                            if filter_obj:
                                                                                filter_obj[0].add(Key(k6, x))
                                                                            else:
                                                                                tmp_obj = CLASS_NAMES[k4](k5)
                                                                                tmp_obj.add(Key(k6, x))
                                                                                nginx_obj.filter(btype=k1.capitalize())[0].filter(
                                                                                btype=k2.capitalize(), name=k3)[0].add(tmp_obj)


        return nginx_obj

    @classmethod
    def load_conf(cls, data, conf=True):
        """
        Load an nginx configuration from a provided string.

        :param str data: nginx configuration
        :param bool conf: Load object(s) into a Conf object?
        """
        f = Conf() if conf else []
        lopen = []
        index = 0

        while True:
            m = re.compile(r'^\s*events\s*{', re.S).search(data[index:])
            if m:
                e = Events()
                lopen.insert(0, e)
                index += m.end()
                continue

            m = re.compile(r'^\s*http\s*{', re.S).search(data[index:])
            if m:
                h = Http()
                lopen.insert(0, h)
                index += m.end()
                continue

            m = re.compile(r'^\s*stream\s*{', re.S).search(data[index:])
            if m:
                s = Stream()
                lopen.insert(0, s)
                index += m.end()
                continue

            m = re.compile(r'^\s*server\s*{', re.S).search(data[index:])
            if m:
                s = Server()
                lopen.insert(0, s)
                index += m.end()
                continue

            m = re.compile(r'^\s*location\s*([^;]*?)\s*{', re.S).search(data[index:])
            if m:
                l = Location(m.group(1))
                lopen.insert(0, l)
                index += m.end()
                continue

            m = re.compile(r'^\s*if\s*([^;]*?)\s*{', re.S).search(data[index:])
            if m:
                ifs = If(m.group(1))
                lopen.insert(0, ifs)
                index += m.end()
                continue

            m = re.compile(r'^\s*upstream\s*([^;]*?)\s*{', re.S).search(data[index:])
            if m:
                u = Upstream(m.group(1))
                lopen.insert(0, u)
                index += m.end()
                continue

            m = re.compile(r'^\s*geo\s*([^;]*?)\s*{', re.S).search(data[index:])
            if m:
                g = Geo(m.group(1))
                lopen.insert(0, g)
                index += m.end()
                continue

            m = re.compile(r'^\s*map\s*([^;]*?)\s*{', re.S).search(data[index:])
            if m:
                g = Map(m.group(1))
                lopen.insert(0, g)
                index += m.end()
                continue

            m = re.compile(r'^\s*limit_except\s*([^;]*?)\s*{', re.S).search(data[index:])
            if m:
                l = Limit_except(m.group(1))
                lopen.insert(0, l)
                index += m.end()
                continue

            m = re.compile(r'^\s*types\s*{', re.S).search(data[index:])
            if m:
                l = Types()
                lopen.insert(0, l)
                index += m.end()
                continue

            m = re.compile(r'^(\s*)#[ \r\t\f]*(.*?)\n', re.S).search(data[index:])
            if m:
                c = Comment(m.group(2), inline='\n' not in m.group(1))
                if lopen and isinstance(lopen[0], Container):
                    lopen[0].add(c)
                else:
                    f.add(c) if conf else f.append(c)
                index += m.end() - 1
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

            double = r'\s*"[^"]*"'
            single = r'\s*\'[^\']*\''
            normal = r'\s*[^;\s]*'
            s1 = r'{}|{}|{}'.format(double, single, normal)
            s = r'^\s*({})\s*((?:{})+);'.format(s1, s1)
            m = re.compile(s, re.S).search(data[index:])
            if m:
                k = Key(m.group(1), m.group(2))
                if lopen and isinstance(lopen[0], (Container, Server)):
                    lopen[0].add(k)
                else:
                    f.add(k) if conf else f.append(k)
                index += m.end()
                continue

            m = re.compile(r'^\s*(\S+);', re.S).search(data[index:])
            if m:
                k = Key(m.group(1), '')
                if lopen and isinstance(lopen[0], (Container, Server)):
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
        Load an nginx configuration from a provided file path.

        :param file path: path to nginx configuration on disk
        """
        with open(path, 'r') as f:
            return cls.load_conf(f.read())

    @classmethod
    def dumps(cls, obj):
        """
        Dump an nginx configuration to a string.

        :param obj obj: nginx object (Conf, Server, Container)
        :returns: nginx configuration as string
        """
        return ''.join(obj.as_strings)

    @classmethod
    def dumpf(cls, obj, path):
        """
        Write an nginx configuration to file.

        :param obj obj: nginx object (Conf, Server, Container)
        :param str path: path to nginx configuration on disk
        :returns: path the configuration was written to
        """
        syslog(MODULE_NAME, "Writing nginx config {0}".format(path))
        with open(path, 'w') as f:
            f.write(cls.dumps(obj))
        return path

    @classmethod
    def get_http_servers(cls):
        """
        return conf.d/http servers, list
        """
        servers = []
        if os.path.exists(HTTP_INCLUDE_DIR):
            for f in os.listdir(HTTP_INCLUDE_DIR):
                if os.path.isfile(os.path.join(HTTP_INCLUDE_DIR, f)):
                    if os.path.splitext(f)[1] == ".conf":
                        servers.append(os.path.splitext(f)[0])
        return servers

    @classmethod
    def get_stream_servers(cls):
        """
        return conf.d/stream servers, list
        """
        servers = []
        if os.path.exists(STREAM_INCLUDE_DIR):
            for f in os.listdir(STREAM_INCLUDE_DIR):
                if os.path.isfile(os.path.join(STREAM_INCLUDE_DIR, f)):
                    if os.path.splitext(f)[1] == ".conf":
                        servers.append(os.path.splitext(f)[0])
        return servers

    @classmethod
    def reset_failed_nginx_service(cls):
        """
        systemctl restart slb-nginx
        """
        cmd = "systemctl reset-failed %s" % SERVICE_NAME
        (outmsg, errmsg) = cmdprocess.shell_cmd(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg)

    @classmethod
    def start_nginx_service(cls):
        """
        systemctl restart slb-nginx
        """
        cmd = "systemctl start %s" % SERVICE_NAME
        (outmsg, errmsg) = cmdprocess.shell_cmd(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg)

    @classmethod
    def restart_nginx_service(cls):
        """
        systemctl restart slb-nginx
        """
        cmd = "systemctl restart %s" % SERVICE_NAME
        (outmsg, errmsg) = cmdprocess.shell_cmd(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg)

    @classmethod
    def stop_nginx_service(cls):
        """stop slb-nginx service"""
        cmd = "systemctl stop %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg)

    @classmethod
    def enable_nginx_service(cls):
        """enable slb-nginx service"""
        cmd = "systemctl enable %s" % SERVICE_NAME
        (outmsg, errmsg) = cmdprocess.shell_cmd(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg)

    @classmethod
    def disable_nginx_service(cls):
        """disable slb-nginx service"""
        cmd = "systemctl disable %s" % SERVICE_NAME
        (outmsg, errmsg) = cmdprocess.shell_cmd(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg)

    @classmethod
    def get_nginx_status(cls):
        """slb-nginx service status"""
        cmd = "systemctl status %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        if returncode == 0:
            return True
        else:
            return False

    @classmethod
    def get_nginx_enabled(cls):
        """slb-nginx is-enabled status"""
        cmd = "systemctl is-enabled %s" % SERVICE_NAME
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        if returncode == 0:
            return True
        else:
            return False

    @classmethod
    def reload_nginx_service(cls):
        """reload slb-nginx service"""
        cmd = "systemctl reload %s" % SERVICE_NAME
        (outmsg, errmsg) = cmdprocess.shell_cmd(cmd)
        syslog_cmd(MODULE_NAME, cmd, outmsg, errmsg)

    @classmethod
    def test_nginx_conf(cls):
        """ slb-nginx -t """
        cmd = "%s -t" % SLB_NGINX_SBIN
        (returncode, outmsg, errmsg) = cmdprocess.shell_cmd_not_raise(cmd)
        if returncode:
            return errmsg
        else:
            return None


def adapte_cpu(nginx_conf, worker_processes):
    import platform
    import multiprocessing
    if platform.uname()[-1] == "aarch64":
        cpunum = multiprocessing.cpu_count()
        if worker_processes == 'auto':
            nums = cpunum
        elif int(worker_processes) >= cpunum:
            nums = cpunum
        else:
            nums = int(worker_processes)
        for k in nginx_conf.children:
            if k.name == 'worker_processes':
                k.value = str(nums)
            if k.name == 'worker_cpu_affinity':
                k.value = cpu_info(nums)
    else:
        for k in nginx_conf.children:
            if k.name == 'worker_processes':
                k.value = worker_processes
    return nginx_conf


def cpu_info(num):
    cpu_info = ""
    for i in range(num):
        cpu_info += "\n" + str(10 ** (i))
    return cpu_info
