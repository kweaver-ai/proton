#!/usr/bin/env python
# -*- coding:utf-8 -*-

""" 
Copyright (C) EISOO Systems, Inc - All Rights Reserved
  Unauthorized copying of this file, via any medium is strictly prohibited
  Proprietary and confidential
  Written by Kelu Tang <tang.kelu@aishu.cn>, 2020
"""
import os
import sys
import json
from tornado.web import RequestHandler

CURR_SCRIPT_PATH = os.path.dirname(os.path.abspath(sys.argv[0]))
sys.path.append(os.path.dirname(CURR_SCRIPT_PATH))

from src.modules.nginx_agent import NginxAgent, adapte_cpu
from src.modules.nginx_agent import Key
from src.modules.pydeps.logger import syslog, syslog_error

NGINX_CONF_PATH = '/usr/local/slb-nginx/conf/nginx.conf'
NGINX_CONF_BAK_PATH = '/usr/local/slb-nginx/conf/nginx.bak'
NGINX_CONF_DEFAULT_PATH = '/usr/local/slb-nginx/conf/nginx.conf.default'
NGINX_ERR_LOG_PATH = '/var/log/slb-nginx/error.log'
NGINX_INCLUDE_DIR = '/usr/local/slb-nginx/conf.d'
HTTP_INCLUDE_DIR = os.path.join(NGINX_INCLUDE_DIR, 'http')
STREAM_INCLUDE_DIR = os.path.join(NGINX_INCLUDE_DIR, 'stream')
MODULE_NAME = "Proton_SLB_Manager"

errmsg = {
    "code": "",
    "message": "",
    "cause": "",
    "detail": ""
}


def check_json(func):
    def wrapper(self, *args, **kwargs):
        try:
            if self.request.body:
                json.loads(self.request.body)
            func(self, *args, **kwargs)
        except:
            errmsg["code"] = "500010000"
            errmsg["message"] = "data format error"
            errmsg["cause"] = "data is not in json format"
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "The incoming data is not in json format")

    return wrapper


class NginxInstanceHandler(RequestHandler):
    def get(self):
        try:
            if os.path.exists(NGINX_CONF_PATH):
                nginx_conf = NginxAgent.loadf(NGINX_CONF_PATH)
                self.write(json.dumps(nginx_conf.as_dict))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx instance not found"
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self):
        try:
            if os.path.exists(NGINX_CONF_PATH):
                errmsg["code"] = "409010000"
                errmsg["message"] = "conflict"
                errmsg["cause"] = "nginx instance already exists"
                self.write(errmsg)
                self.set_status(409)
            else:
                if not os.path.exists(os.path.dirname(NGINX_ERR_LOG_PATH)):
                    os.makedirs(os.path.dirname(NGINX_ERR_LOG_PATH))
                if not os.path.exists(NGINX_ERR_LOG_PATH):
                    open(NGINX_ERR_LOG_PATH, 'a').close()

                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf']:
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = "need conf section"
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create nginx instance failed, {0}".format(errmsg["cause"]))
                    return
                nginx_conf = NginxAgent.loadf(NGINX_CONF_DEFAULT_PATH)

                if data['conf'].has_key('worker_processes'):
                    worker_processes = data['conf']['worker_processes']

                nginx_conf = adapte_cpu(nginx_conf, worker_processes)
                NginxAgent.dumpf(nginx_conf, NGINX_CONF_PATH)
                msg = NginxAgent.test_nginx_conf()
                if msg:
                    os.remove(NGINX_CONF_PATH)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create nginx instance failed, {0}".format(msg))
                else:
                    NginxAgent.enable_nginx_service()
                    NginxAgent.restart_nginx_service()
                    self.set_status(201)
                    syslog(MODULE_NAME, "Create nginx instance success")
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create nginx instance failed, {0}".format(ex.message))

    def delete(self):
        try:
            # 先删除配置文件，防止被proton_slb_manager的守护线程启动
            if os.path.exists(NGINX_CONF_PATH):
                os.remove(NGINX_CONF_PATH)
            NginxAgent.stop_nginx_service()
            NginxAgent.disable_nginx_service()
            self.set_status(204)
            syslog(MODULE_NAME, "Delete nginx instance success")
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Delete nginx instance failed, {0}".format(ex.message))

    @check_json
    def put(self):
        try:
            if not os.path.exists(NGINX_CONF_PATH):
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx instance not found"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set nginx instance failed, {0}".format(errmsg["cause"]))
            else:
                # convert unicode to utf8
                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf']:
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = "need conf section"
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set nginx instance failed, {0}".format(errmsg["cause"]))
                    return

                nginx_conf = NginxAgent.load_json(data)

                # back nginx.conf and test it
                if os.path.exists(NGINX_CONF_BAK_PATH):
                    os.remove(NGINX_CONF_BAK_PATH)
                os.rename(NGINX_CONF_PATH, NGINX_CONF_BAK_PATH)
                NginxAgent.dumpf(nginx_conf, NGINX_CONF_PATH)
                msg = NginxAgent.test_nginx_conf()
                if msg:
                    os.remove(NGINX_CONF_PATH)
                    os.rename(NGINX_CONF_BAK_PATH, NGINX_CONF_PATH)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set nginx instance failed, {0}".format(errmsg["cause"]))
                else:
                    NginxAgent.dumpf(nginx_conf, NGINX_CONF_PATH)
                    NginxAgent.reset_failed_nginx_service()
                    NginxAgent.restart_nginx_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Set nginx instance success")
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set nginx instance failed, {0}".format(ex.message))


class NginxHttpHandler(RequestHandler):
    def get(self):
        try:
            servers = NginxAgent.get_http_servers()
            self.write(json.dumps(servers))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.set_status(500)

    @check_json
    def post(self):
        try:
            data = eval(self.request.body)
            if not data.has_key('name') or not data['name']:
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need name section"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Create nginx http server failed, {0}".format(errmsg["cause"]))
                return
            if not data.has_key('conf') or not data['conf']:
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need conf section"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Create nginx server failed, {0}".format(errmsg["cause"]))
                return
            server_file = os.path.join(HTTP_INCLUDE_DIR, data['name'] + '.conf')
            if not os.path.exists(HTTP_INCLUDE_DIR):
                os.mkdir(HTTP_INCLUDE_DIR)
            if os.path.exists(server_file):
                errmsg["code"] = "409010000"
                errmsg["message"] = "conflict"
                errmsg["cause"] = "http server {0} already exists".format(data['name'])
                self.write(errmsg)
                self.set_status(409)
                syslog_error(MODULE_NAME, "Create nginx http server failed, {0}".format(errmsg["cause"]))
                return
            else:
                nginx_conf = NginxAgent.load_json(data)
                NginxAgent.dumpf(nginx_conf, server_file)
                msg = NginxAgent.test_nginx_conf()
                if msg:
                    os.remove(server_file)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create nginx http server failed, {0}".format(errmsg["cause"]))
                    return
                else:
                    NginxAgent.reset_failed_nginx_service()
                    NginxAgent.reload_nginx_service()
                    self.set_status(201)
                    syslog(MODULE_NAME, "Create nginx http server success")
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create nginx http server failed, {0}".format(ex.message))


class NginxHttpServerHandler(RequestHandler):
    def get(self, servername):
        try:
            server_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                self.write(json.dumps(nginx_conf.as_dict))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx http server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def put(self, servername):
        try:
            data = eval(self.request.body)
            if not data.has_key('conf') or not data['conf']:
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need conf section"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Set nginx http server {0} failed, {1}".format(servername, errmsg["cause"]))
                return
            server_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.conf')
            if not os.path.exists(server_file):
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx http server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set nginx http server {0} failed, {1}".format(servername, errmsg["cause"]))
            else:
                tmp_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.tmpfile')
                if os.path.exists(tmp_file):
                    os.remove(tmp_file)
                os.rename(server_file, tmp_file)
                nginx_conf = NginxAgent.load_json(data)
                NginxAgent.dumpf(nginx_conf, server_file)
                msg = NginxAgent.test_nginx_conf()
                if msg:
                    os.remove(server_file)
                    os.rename(tmp_file, server_file)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME,
                                 "Set nginx http server {0} failed, {1}".format(servername, errmsg["cause"]))
                else:
                    NginxAgent.dumpf(nginx_conf, server_file)
                    NginxAgent.reset_failed_nginx_service()
                    NginxAgent.reload_nginx_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Set nginx http server success")
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set nginx http server {0} failed, {1}".format(servername, errmsg["cause"]))

    def delete(self, servername):
        try:
            server_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                os.remove(server_file)
                NginxAgent.reset_failed_nginx_service()
                NginxAgent.reload_nginx_service()
            self.set_status(204)
            syslog(MODULE_NAME, "Delete nginx server {0} success".format(servername))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Delete nginx server {0} failed, {1}".format(servername, errmsg["cause"]))


class NginxHttpUpstreamHandler(RequestHandler):
    def get(self, servername):
        try:
            upstreams = []
            server_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                upstream_obj = nginx_conf.filter(btype='Upstream')
                if upstream_obj:
                    for obj in upstream_obj:
                        upstreams.append(obj.value)
                self.write(json.dumps(upstreams))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx http server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self, servername):
        try:
            data = eval(self.request.body)
            if not data.has_key('conf') or not data['conf'] or not data['conf']['upstream'].keys():
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need conf upstream name"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME,
                             "Create nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))
                return
            server_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.conf')
            if not os.path.exists(server_file):
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx http server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME,
                             "Create nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))
            else:
                nginx_conf = NginxAgent.loadf(server_file)
                new_upstream = data['conf']['upstream'].keys()
                for i in new_upstream:
                    if nginx_conf.filter(btype='Upstream', name=i):
                        errmsg["code"] = "409010000"
                        errmsg["message"] = "conflict"
                        errmsg["cause"] = "already exist upstream {0}, all request abort".format(i)
                        self.write(errmsg)
                        self.set_status(409)
                        syslog_error(MODULE_NAME,
                                     "Create nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))
                        return
                upstream_obj = NginxAgent.load_json(data)
                for obj in upstream_obj.filter(btype='Upstream'):
                    nginx_conf.add(obj)

                tmp_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.tmpfile')
                if os.path.exists(tmp_file):
                    os.remove(tmp_file)
                os.rename(server_file, tmp_file)
                NginxAgent.dumpf(nginx_conf, server_file)
                msg = NginxAgent.test_nginx_conf()
                if msg:
                    os.remove(server_file)
                    os.rename(tmp_file, server_file)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME,
                                 "Create nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))
                else:
                    NginxAgent.reset_failed_nginx_service()
                    NginxAgent.reload_nginx_service()
                    self.set_status(201)
                    syslog(MODULE_NAME, "Create nginx http upstream success")
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create nginx http upstream failed, {0}".format(ex.message))


class NginxHttpServerUpstreamHandler(RequestHandler):
    def get(self, servername, upstreamname):
        try:
            server_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                upstream_obj = nginx_conf.filter(btype='Upstream', name=upstreamname)
                if upstream_obj:
                    conf_dict = {'conf': upstream_obj[0].as_dict}
                    self.write(json.dumps(conf_dict))
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "upstream {0} in server {1} not found".format(upstreamname, servername)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self, servername, upstreamname):
        try:
            data = eval(self.request.body)
            if not data.has_key('conf') or not data['conf'] or not data['conf']['ip']:
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need conf ip section"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Add nginx http upstream ip failed, {0}".format(errmsg["cause"]))
                return
            server_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                upstream_obj = nginx_conf.filter(btype='Upstream', name=upstreamname)
                if upstream_obj:
                    real_server = data['conf']['ip'] + ':' + data['conf']['port'] + ' ' + data['conf']['extra']
                    upstream_obj[0].add(Key('server', real_server))

                    tmp_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.tmpfile')
                    if os.path.exists(tmp_file):
                        os.remove(tmp_file)
                    os.rename(server_file, tmp_file)
                    NginxAgent.dumpf(nginx_conf, server_file)
                    msg = NginxAgent.test_nginx_conf()
                    if msg:
                        os.remove(server_file)
                        os.rename(tmp_file, server_file)
                        errmsg["code"] = "400010000"
                        errmsg["message"] = "invalid input"
                        errmsg["cause"] = msg
                        self.write(errmsg)
                        self.set_status(400)
                        syslog_error(MODULE_NAME, "Add nginx http upstream ip failed, {0}".format(errmsg["cause"]))
                        return
                    else:
                        NginxAgent.reset_failed_nginx_service()
                        NginxAgent.reload_nginx_service()
                    self.set_status(201)
                    syslog(MODULE_NAME, "Add nginx http upstream ip success")
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "upstream {0} in server {1} not found".format(upstreamname, servername)
                    self.write(errmsg)
                    self.set_status(404)
                    syslog_error(MODULE_NAME, "Add nginx http upstream ip failed, {0}".format(errmsg["cause"]))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Add nginx http upstream ip failed, {0}".format(errmsg["cause"]))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Add nginx http upstream ip failed, {0}".format(errmsg["cause"]))

    @check_json
    def put(self, servername, upstreamname):
        try:
            data = eval(self.request.body)
            if not data.has_key('conf') or not data['conf']:
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need conf section"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Set nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))
                return
            server_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                new_upstream = NginxAgent.load_json(data)
                upstream_obj = nginx_conf.filter(btype='Upstream', name=upstreamname)
                if upstream_obj:
                    nginx_conf.remove(upstream_obj[0])
                    nginx_conf.add(new_upstream.filter(btype='Upstream')[0])

                    tmp_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.tmpfile')
                    if os.path.exists(tmp_file):
                        os.remove(tmp_file)
                    os.rename(server_file, tmp_file)
                    NginxAgent.dumpf(nginx_conf, server_file)
                    msg = NginxAgent.test_nginx_conf()
                    if msg:
                        os.remove(server_file)
                        os.rename(tmp_file, server_file)
                        errmsg["code"] = "400010000"
                        errmsg["message"] = "invalid input"
                        errmsg["cause"] = msg
                        self.write(errmsg)
                        self.set_status(400)
                        syslog_error(MODULE_NAME,
                                     "Set nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))
                    else:
                        NginxAgent.dumpf(nginx_conf, server_file)
                        NginxAgent.reset_failed_nginx_service()
                        NginxAgent.reload_nginx_service()
                        self.set_status(204)
                        syslog(MODULE_NAME, "Set nginx http upstream success")
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "upstream {0} in server {1} not found".format(upstreamname, servername)
                    self.write(errmsg)
                    self.set_status(404)
                    syslog_error(MODULE_NAME,
                                 "Set nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))

    def delete(self, servername, upstreamname):
        try:
            server_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                upstream_obj = nginx_conf.filter(btype='Upstream', name=upstreamname)
                if upstream_obj:
                    nginx_conf.remove(upstream_obj[0])
                    NginxAgent.dumpf(nginx_conf, server_file)
                    NginxAgent.reset_failed_nginx_service()
                    NginxAgent.reload_nginx_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Delete nginx http upstream {0} success".format(servername))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME,
                             "Delete nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Delete nginx http upstream {0} failed, {1}".format(servername, errmsg["cause"]))


class NginxHttpServerUpstreamIpHandler(RequestHandler):
    def delete(self, servername, upstreamname, ip):
        try:
            server_file = os.path.join(HTTP_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                upstream_obj = nginx_conf.filter(btype='Upstream', name=upstreamname)
                if upstream_obj:
                    for k in upstream_obj[0].children:
                        if k.name == 'server':
                            if ip in k.value:
                                upstream_obj[0].remove(k)
                    NginxAgent.dumpf(nginx_conf, server_file)
                    NginxAgent.reset_failed_nginx_service()
                    NginxAgent.reload_nginx_service()
                    self.set_status(204)
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "upstream {0} in server {1} not found".format(upstreamname, servername)
                    self.write(errmsg)
                    self.set_status(404)
                    syslog_error(MODULE_NAME, "Delete nginx http upstream ip {0} failed, {0}".format(errmsg["cause"]))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Delete nginx http upstream ip failed, {0}".format(errmsg["cause"]))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Delete nginx http upstream ip failed, {0}".format(errmsg["cause"]))


class NginxStreamHandler(RequestHandler):
    def get(self):
        try:
            servers = NginxAgent.get_stream_servers()
            self.write(json.dumps(servers))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self):
        try:
            data = eval(self.request.body)
            if not data.has_key('name') or not data['name']:
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need name section"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Create nginx stream server failed, {0}".format(errmsg["cause"]))
                return
            if not data.has_key('conf') or not data['conf']:
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need conf section"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Create nginx stream server failed, {0}".format(errmsg["cause"]))
                return
            server_file = os.path.join(STREAM_INCLUDE_DIR, data['name'] + '.conf')
            if not os.path.exists(STREAM_INCLUDE_DIR):
                os.mkdir(STREAM_INCLUDE_DIR)
            if os.path.exists(server_file):
                errmsg["code"] = "409010000"
                errmsg["message"] = "conflict"
                errmsg["cause"] = "stream server {0} already exists".format(data['name'])
                self.write(errmsg)
                self.set_status(409)
                syslog_error(MODULE_NAME, "Create nginx stream server failed, {0}".format(errmsg["cause"]))
            else:
                nginx_conf = NginxAgent.load_json(data)
                NginxAgent.dumpf(nginx_conf, server_file)
                msg = NginxAgent.test_nginx_conf()
                if msg:
                    os.remove(server_file)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create nginx stream server failed, {0}".format(errmsg["cause"]))
                else:
                    NginxAgent.reset_failed_nginx_service()
                    NginxAgent.reload_nginx_service()
                    self.set_status(201)
                    syslog(MODULE_NAME, "Create nginx stream server success")
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create nginx stream server failed, {0}".format(errmsg["cause"]))


class NginxStreamServerHandler(RequestHandler):
    def get(self, servername):
        try:
            server_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                self.write(json.dumps(nginx_conf.as_dict))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def put(self, servername):
        try:
            data = eval(self.request.body)
            if not data.has_key('conf') or not data['conf']:
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need conf section"
                self.write(errmsg)
                self.set_status(500)
                syslog_error(MODULE_NAME, "Set nginx stream server {0} failed, {1}".format(servername, errmsg["cause"]))
                return
            server_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.conf')
            if not os.path.exists(server_file):
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set nginx stream server {0} failed, {1}".format(servername, errmsg["cause"]))
            else:
                nginx_conf = NginxAgent.load_json(data)
                tmp_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.tmpfile')
                if os.path.exists(tmp_file):
                    os.remove(tmp_file)
                os.rename(server_file, tmp_file)
                NginxAgent.dumpf(nginx_conf, server_file)
                msg = NginxAgent.test_nginx_conf()
                if msg:
                    os.remove(server_file)
                    os.rename(tmp_file, server_file)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME,
                                 "Set nginx stream server {0} failed, {1}".format(servername, errmsg["cause"]))
                else:
                    NginxAgent.dumpf(nginx_conf, server_file)
                    NginxAgent.reset_failed_nginx_service()
                    NginxAgent.reload_nginx_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Set nginx stream server success")
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set nginx stream server {0} failed, {1}".format(servername, errmsg["cause"]))

    def delete(self, servername):
        try:
            server_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                os.remove(server_file)
                NginxAgent.reset_failed_nginx_service()
                NginxAgent.reload_nginx_service()
            self.set_status(204)
            syslog(MODULE_NAME, "Delete nginx server {0} success".format(servername))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set nginx stream server {0} failed, {1}".format(servername, errmsg["cause"]))


class NginxStreamUpstreamHandler(RequestHandler):
    def get(self, servername):
        try:
            upstreams = []
            server_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                upstream_obj = nginx_conf.filter(btype='Upstream')
                if upstream_obj:
                    for obj in upstream_obj:
                        upstreams.append(obj.value)
                self.write(json.dumps(upstreams))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self, servername):
        try:
            data = eval(self.request.body)
            if not data.has_key('conf') or not data['conf'] or not data['conf']['upstream'].keys():
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need conf upstream name"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME,
                             "Create nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))
                return
            server_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.conf')
            if not os.path.exists(server_file):
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME,
                             "Create nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))
            else:
                nginx_conf = NginxAgent.loadf(server_file)
                new_upstream = data['conf']['upstream'].keys()
                for i in new_upstream:
                    if nginx_conf.filter(btype='Upstream', name=i):
                        errmsg["code"] = "409010000"
                        errmsg["message"] = "conflict"
                        errmsg["cause"] = "already exist upstream {0}, all request abort".format(i)
                        self.write(errmsg)
                        self.set_status(409)
                        syslog_error(MODULE_NAME,
                                     "Create nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))
                upstream_obj = NginxAgent.load_json(data)
                for obj in upstream_obj.filter(btype='Upstream'):
                    nginx_conf.add(obj)

                tmp_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.tmpfile')
                if os.path.exists(tmp_file):
                    os.remove(tmp_file)
                os.rename(server_file, tmp_file)
                NginxAgent.dumpf(nginx_conf, server_file)
                msg = NginxAgent.test_nginx_conf()
                if msg:
                    os.remove(server_file)
                    os.rename(tmp_file, server_file)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME,
                                 "Create nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))
                else:
                    NginxAgent.reset_failed_nginx_service()
                    NginxAgent.reload_nginx_service()
                    self.set_status(201)
                    syslog(MODULE_NAME, "Create nginx stream upstream {0} success".format(servername))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Create nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))


class NginxStreamServerUpstreamHandler(RequestHandler):
    def get(self, servername, upstreamname):
        try:
            server_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                upstream_obj = nginx_conf.filter(btype='Upstream', name=upstreamname)
                if upstream_obj:
                    conf_dict = {'conf': upstream_obj[0].as_dict}
                    self.write(json.dumps(conf_dict))
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "upstream {0} in server {1} not found".format(upstreamname, servername)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self, servername, upstreamname):
        try:
            data = eval(self.request.body)
            if not data.has_key('conf') or not data['conf'] or not data['conf']['ip']:
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need conf ip section"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Add nginx stream upstream ip failed, {0}".format(errmsg["cause"]))
                return
            server_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                upstream_obj = nginx_conf.filter(btype='Upstream', name=upstreamname)
                if upstream_obj:
                    real_server = data['conf']['ip'] + ':' + data['conf']['port'] + ' ' + data['conf']['extra']
                    upstream_obj[0].add(Key('server', real_server))

                    tmp_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.tmpfile')
                    if os.path.exists(tmp_file):
                        os.remove(tmp_file)
                    os.rename(server_file, tmp_file)
                    NginxAgent.dumpf(nginx_conf, server_file)
                    msg = NginxAgent.test_nginx_conf()
                    if msg:
                        os.remove(server_file)
                        os.rename(tmp_file, server_file)
                        errmsg["code"] = "400010000"
                        errmsg["message"] = "invalid input"
                        errmsg["cause"] = msg
                        self.write(errmsg)
                        self.set_status(400)
                        syslog_error(MODULE_NAME, "Add nginx stream upstream ip failed, {0}".format(errmsg["cause"]))
                    else:
                        NginxAgent.reset_failed_nginx_service()
                        NginxAgent.reload_nginx_service()
                        self.set_status(201)
                        syslog(MODULE_NAME, "Add nginx stream upstream ip success")
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "upstream {0} in server {1} not found".format(upstreamname, servername)
                    self.write(errmsg)
                    self.set_status(404)
                    syslog_error(MODULE_NAME, "Add nginx stream upstream ip failed, {0}".format(errmsg["cause"]))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Add nginx stream upstream ip failed, {0}".format(errmsg["cause"]))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Add nginx stream upstream ip failed, {0}".format(errmsg["cause"]))

    @check_json
    def put(self, servername, upstreamname):
        try:
            data = eval(self.request.body)
            if not data.has_key('conf') or not data['conf']:
                errmsg["code"] = "400010000"
                errmsg["message"] = "invalid input"
                errmsg["cause"] = "need conf section"
                self.write(errmsg)
                self.set_status(500)
                syslog_error(MODULE_NAME,
                             "Set nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))
                return
            server_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                new_upstream = NginxAgent.load_json(data)
                upstream_obj = nginx_conf.filter(btype='Upstream', name=upstreamname)
                if upstream_obj:
                    nginx_conf.remove(upstream_obj[0])
                    nginx_conf.add(new_upstream.filter(btype='Upstream')[0])

                    tmp_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.tmpfile')
                    if os.path.exists(tmp_file):
                        os.remove(tmp_file)
                    os.rename(server_file, tmp_file)
                    NginxAgent.dumpf(nginx_conf, server_file)
                    msg = NginxAgent.test_nginx_conf()
                    if msg:
                        os.remove(server_file)
                        os.rename(tmp_file, server_file)
                        errmsg["code"] = "400010000"
                        errmsg["message"] = "invalid input"
                        errmsg["cause"] = msg
                        self.write(errmsg)
                        self.set_status(400)
                        syslog_error(MODULE_NAME,
                                     "Set nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))
                    else:
                        NginxAgent.dumpf(nginx_conf, server_file)
                        NginxAgent.reset_failed_nginx_service()
                        NginxAgent.reload_nginx_service()
                        self.set_status(204)
                        syslog(MODULE_NAME, "Set nginx stream upstream success")
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "upstream {0} in server {1} not found".format(upstreamname, servername)
                    self.write(errmsg)
                    self.set_status(404)
                    syslog_error(MODULE_NAME,
                                 "Set nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME,
                             "Set nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))

    def delete(self, servername, upstreamname):
        try:
            server_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                upstream_obj = nginx_conf.filter(btype='Upstream', name=upstreamname)
                if upstream_obj:
                    nginx_conf.remove(upstream_obj[0])
                    NginxAgent.dumpf(nginx_conf, server_file)
                    NginxAgent.reset_failed_nginx_service()
                    NginxAgent.reload_nginx_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Delete nginx stream upstream success")
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME,
                             "Delete nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Delete nginx stream upstream {0} failed, {1}".format(servername, errmsg["cause"]))


class NginxStreamServerUpstreamIpHandler(RequestHandler):
    def delete(self, servername, upstreamname, ip):
        try:
            server_file = os.path.join(STREAM_INCLUDE_DIR, servername + '.conf')
            if os.path.exists(server_file):
                nginx_conf = NginxAgent.loadf(server_file)
                upstream_obj = nginx_conf.filter(btype='Upstream', name=upstreamname)
                if upstream_obj:
                    for k in upstream_obj[0].children:
                        if k.name == 'server':
                            if ip in k.value:
                                upstream_obj[0].remove(k)
                                NginxAgent.dumpf(nginx_conf, server_file)
                                NginxAgent.reset_failed_nginx_service()
                                NginxAgent.reload_nginx_service()
                                break
                    self.set_status(204)
                    syslog(MODULE_NAME, "Delete nginx stream upstream ip success")
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "upstream {0} in server {1} not found".format(upstreamname, servername)
                    self.write(errmsg)
                    self.set_status(404)
                    syslog_error(MODULE_NAME,
                                 "Delete nginx stream upstream {0} ip failed, {1}".format(servername, errmsg["cause"]))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "nginx stream server {0} not found".format(servername)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME,
                             "Delete nginx stream upstream {0} ip failed, {1}".format(servername, errmsg["cause"]))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Delete nginx stream upstream {0} ip failed, {1}".format(servername, errmsg["cause"]))
