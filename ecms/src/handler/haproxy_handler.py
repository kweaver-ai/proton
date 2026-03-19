#!/usr/bin/env python
# -*- coding:utf-8 -*-

""" 
Copyright (C) AISHU Systems, Inc - All Rights Reserved
  Unauthorized copying of this file, via any medium is strictly prohibited
  Proprietary and confidential
  Written by Kelu Tang <tang.kelu@aishu.cn>, 2022.05
"""
from ast import Not
import os
import sys
import json
import shutil
from collections import Counter
from wsgiref import validate
from tornado.web import RequestHandler

CURR_SCRIPT_PATH = os.path.dirname(os.path.abspath(sys.argv[0]))
sys.path.append(os.path.dirname(CURR_SCRIPT_PATH))

from src.modules.pydeps.logger import syslog, syslog_error
from src.modules.haproxy_agent import Conf, Frontend, Backend, HAProxyAgent, Global, Defaults, Key

HAPROXY_CONFIG = "/usr/local/haproxy/haproxy.cfg"
HAPROXY_CONFIG_BAK = "/usr/local/haproxy/haproxy.cfg.tmpfile"
HAPROXY_CONFIG_ERR = "/usr/local/haproxy/haproxy.cfg.error"
MODULE_NAME = "Proton_SLB_Manager"
errmsg = {
    "ErrorCode": "",
    "Description": "",
    "Solution": "Please contact the vendor.",
    "ErrorDetails": "",
    "ErrorLink": ""
}

def check_json(func):
    def wrapper(self, *args, **kwargs):
        try:
            if self.request.body:
                json.loads(self.request.body)
            func(self, *args, **kwargs)
        except:
            errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
            errmsg["Description"] = "data format error"
            errmsg["ErrorDetails"] = "data is not in json format"
            self.write(errmsg)
            self.set_status(400)
            syslog_error(MODULE_NAME, "The incoming data is not in json format")

    return wrapper


class HAProxyConfigHandler(RequestHandler):
    def get(self):
        try:
            if os.path.getsize(HAPROXY_CONFIG):
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                self.write(json.dumps(haproxy_conf.as_dict))
            else:
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFoundOrEmpty."
                errmsg["Description"] = "no such file haproxy.cfg or file is empty."
                errmsg["ErrorDetails"] = "no such file haproxy.cfg or file is empty."
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self):
        try:
            if not os.path.exists(HAPROXY_CONFIG):
                open(HAPROXY_CONFIG, 'w').close()
            if not os.path.getsize(HAPROXY_CONFIG):
                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf'] or not data['conf'].has_key('frontend') or not data['conf'].has_key('backend') \
                    or not data['conf'].has_key('global') or not data['conf'].has_key('defaults'):
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input json data format error, like {'conf': {'global': {}, 'defaults': {}, 'frontend': {}, 'backend': {}}}, input {0}".format(data)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create haproxy config failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                if not data['conf']['global'] or not data['conf']['defaults'] or not data['conf']['frontend'] or not data['conf']['backend']:
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input data global/defaults/frontend/backend is empty, input is {0}.".format(data)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create haproxy config failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                
                haproxy_conf = HAProxyAgent.load_json(data)

                # backup haproxy.cfg and test it
                if os.path.exists(HAPROXY_CONFIG_BAK):
                    os.remove(HAPROXY_CONFIG_BAK)
                if os.path.exists(HAPROXY_CONFIG):
                    os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_BAK)
                HAProxyAgent.dumpf(haproxy_conf, HAPROXY_CONFIG)
                msg, returncode = HAProxyAgent.test_haproxy_conf()
                if msg:
                    os.remove(HAPROXY_CONFIG)
                    os.rename(HAPROXY_CONFIG_BAK, HAPROXY_CONFIG)
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create haproxy config {0} failed, {1}".format(data, errmsg["ErrorDetails"]))
                else:
                    HAProxyAgent.restart_haproxy_service()
                    HAProxyAgent.enable_haproxy_service()
                    self.set_status(201)
                    syslog(MODULE_NAME, "Create haproxy config {0} success".format(data))
            else:
                errmsg["ErrorCode"] = "SLB.HAProxy.Conflict"
                errmsg["Description"] = "input conflict"
                errmsg["ErrorDetails"] = "haproxy.cfg is not empty, please call PUT method."
                self.write(errmsg)
                self.set_status(409)
                syslog_error(MODULE_NAME, "Create haproxy config failed, {0}".format(errmsg["ErrorDetails"]))
                return
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create haproxy config failed, {0}".format(ex.message))

    @check_json
    def put(self):
        try:
            if os.path.getsize(HAPROXY_CONFIG):
                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf'] or not data['conf'].has_key('frontend') or not data['conf'].has_key('backend') \
                    or not data['conf'].has_key('global') or not data['conf'].has_key('defaults'):
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input json data format error, like {'conf': {'global': {}, 'defaults': {}, 'frontend': {}, 'backend': {}}}, input {0}".format(data)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy config failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                if not data['conf']['global'] or not data['conf']['defaults'] or not data['conf']['frontend'] or not data['conf']['backend']:
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input data global/defaults/frontend/backend is empty, input is {0}.".format(data)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy config failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                
                haproxy_conf = HAProxyAgent.load_json(data)

                # backup haproxy.cfg and test it
                if os.path.exists(HAPROXY_CONFIG_BAK):
                    os.remove(HAPROXY_CONFIG_BAK)
                if os.path.exists(HAPROXY_CONFIG_ERR):
                    os.remove(HAPROXY_CONFIG_ERR)
                if os.path.exists(HAPROXY_CONFIG):
                    os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_BAK)
                HAProxyAgent.dumpf(haproxy_conf, HAPROXY_CONFIG)
                msg, returncode = HAProxyAgent.test_haproxy_conf()
                if msg:
                    os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_ERR)
                    os.rename(HAPROXY_CONFIG_BAK, HAPROXY_CONFIG)
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy config {0} failed, {1}".format(data, errmsg["ErrorDetails"]))
                else:
                    HAProxyAgent.reset_failed_haproxy_service()
                    HAProxyAgent.restart_haproxy_service()
                    HAProxyAgent.enable_haproxy_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Set haproxy config {0} success".format(data))
            else:
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound"
                errmsg["Description"] = "not found"
                errmsg["ErrorDetails"] = "haproxy.cfg is empty, please call POST method."
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set haproxy config failed, {0}".format(errmsg["ErrorDetails"]))
                return
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set haproxy config failed, {0}".format(ex.message))

    def delete(self):
        try:
            if os.path.getsize(HAPROXY_CONFIG):
                HAProxyAgent.clear_haproxy()
                HAProxyAgent.disable_haproxy_service()
                HAProxyAgent.stop_haproxy_service()
            self.set_status(204)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set haproxy config failed, {0}".format(ex.message))

class HAProxyInstancesHandler(RequestHandler):
    def get(self):
        try:
            instances = []
            if os.path.getsize(HAPROXY_CONFIG):
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                frontend_objs = haproxy_conf.filter(btype='Frontend')
                if frontend_objs:
                    backend_objs = haproxy_conf.filter(btype='Backend')
                    for f_obj in frontend_objs:
                        for b_obj in backend_objs:
                            if f_obj.value == b_obj.value:
                                instances.append(f_obj.value.strip())
                self.write(json.dumps(instances))
            else:
                # errmsg["ErrorCode"] = "404010000"
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFoundOrEmpty."
                errmsg["Description"] = "no such file haproxy.cfg or file is empty."
                errmsg["ErrorDetails"] = "no such file haproxy.cfg or file is empty."
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self):
        try:
            data = eval(self.request.body)
            if not data.has_key('conf') or not data['conf'] or not data['conf'].has_key('frontend') \
                or not data['conf'].has_key('backend') or not data['conf']['frontend'] or not data['conf']['backend']:
                errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                errmsg["Description"] = "invalid input"
                errmsg["ErrorDetails"] = "input json data need frontend-backend pair format, like {'conf': {'frontend': {}, 'backend': {}}}"
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
                return
            if dict(Counter(data['conf']['frontend'].keys())) != dict(Counter(data['conf']['backend'].keys())):
                errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                errmsg["Description"] = "invalid input"
                errmsg["ErrorDetails"] = "input frontend name must same with backend."
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}.".format(errmsg["ErrorDetails"]))
                return
            
            haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
            global_obj = haproxy_conf.filter(btype='Global')
            defaults_obj = haproxy_conf.filter(btype='Defaults')
            front_names = data["conf"]["frontend"].keys()
            for i in front_names:
                if haproxy_conf.filter(btype='Frontend', name=i):
                    errmsg["ErrorCode"] = "SLB.HAProxy.Conflict"
                    errmsg["Description"] = "input conflict"
                    errmsg["ErrorDetails"] = "frontend {0} is already exist".format(i)
                    self.write(errmsg)
                    self.set_status(409)
                    syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}.".format(errmsg["ErrorDetails"]))
                    return
            back_names = data["conf"]["backend"].keys()
            for i in back_names:
                if haproxy_conf.filter(btype='Backend', name=i):
                    errmsg["ErrorCode"] = "SLB.HAProxy.Conflict"
                    errmsg["Description"] = "input conflict"
                    errmsg["ErrorDetails"] = "backend {0} is already exist".format(i)
                    self.write(errmsg)
                    self.set_status(409)
                    syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}.".format(errmsg["ErrorDetails"]))
                    return

            # auto initial global and defaults config
            if not global_obj or not defaults_obj:
                global_obj = Global()
                global_obj.add(Key("maxconn", "10000"))
                global_obj.add(Key("log", "/dev/log local0"))
                global_obj.add(Key("user", "haproxy"))
                global_obj.add(Key("group", "haproxy"))
                defaults_obj = Defaults()
                defaults_obj.add(Key("maxconn", "1000"))
                defaults_obj.add(Key("mode", "tcp"))
                defaults_obj.add(Key("option", "dontlognull"))
                defaults_obj.add(Key("timeout", "http-request 10s"))
                defaults_obj.add(Key("timeout", "queue        1m"))
                defaults_obj.add(Key("timeout", "connect      10s"))
                defaults_obj.add(Key("timeout", "client       86400s"))
                defaults_obj.add(Key("timeout", "server       86400s"))
                defaults_obj.add(Key("timeout", "tunnel       86400s"))
                haproxy_conf.add(global_obj)
                haproxy_conf.add(defaults_obj)

            for f_data in data["conf"]["frontend"].keys():
                front_obj = Frontend(f_data)
                haproxy_conf.add(front_obj)
                if data["conf"]["frontend"][f_data]:
                    for k in data["conf"]["frontend"][f_data].keys():
                        front_obj.add(Key(k, data["conf"]["frontend"][f_data][k]))
                else:
                    # invalid data
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input data frontend {0} is empty.".format(f_data)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
            for f_data in data["conf"]["backend"].keys():
                back_obj = Backend(f_data)
                haproxy_conf.add(back_obj)
                if data["conf"]["backend"][f_data]:
                    for k, v in data["conf"]["backend"][f_data].items():
                        if isinstance(v, list):
                            for v1 in v:
                                back_obj.add(Key(k, v1))
                        else:
                            back_obj.add(Key(k, v))
                else:
                    # invalid data
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input data backend {0} is empty.".format(f_data)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))

            if os.path.exists(HAPROXY_CONFIG_BAK):
                os.remove(HAPROXY_CONFIG_BAK)
            if os.path.exists(HAPROXY_CONFIG_ERR):
                os.remove(HAPROXY_CONFIG_ERR)
            os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_BAK)
            HAProxyAgent.dumpf(haproxy_conf, HAPROXY_CONFIG)
            msg, returncode = HAProxyAgent.test_haproxy_conf()
            if msg:
                os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_ERR)
                os.rename(HAPROXY_CONFIG_BAK, HAPROXY_CONFIG)
                errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                errmsg["Description"] = "invalid input"
                errmsg["ErrorDetails"] = msg
                self.write(errmsg)
                self.set_status(400)
                syslog_error(MODULE_NAME, "Create haproxy instance {0} failed, {1}".format(data, errmsg["ErrorDetails"]))
            else:
                HAProxyAgent.restart_haproxy_service()
                HAProxyAgent.enable_haproxy_service()
                self.set_status(201)
                syslog(MODULE_NAME, "Create haproxy instance {0} success".format(data))
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(ex.message))

class HAProxyInstanceHandler(RequestHandler):
    def get(self, instance_name):
        try:
            if os.path.getsize(HAPROXY_CONFIG):
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                frontend_obj = haproxy_conf.filter(btype='Frontend', name=instance_name)
                if frontend_obj:
                    instance_conf = Conf()
                    backend_obj = haproxy_conf.filter(btype='Backend', name=instance_name)
                    if backend_obj:
                        instance_conf.add(frontend_obj[0])
                        instance_conf.add(backend_obj[0])
                        self.write(json.dumps(instance_conf.as_dict))
                    else:
                        errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                        errmsg["Description"] = "not found."
                        errmsg["ErrorDetails"] = "backend {0} not found in haproxy.cfg.".format(instance_name)
                        self.write(errmsg)
                        self.set_status(404)
                else:
                    errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                    errmsg["Description"] = "not found."
                    errmsg["ErrorDetails"] = "frontend {0} not found in haproxy.cfg.".format(instance_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFoundOrEmpty."
                errmsg["Description"] = "no such file haproxy.cfg or file is empty."
                errmsg["ErrorDetails"] = "no such file haproxy.cfg or file is empty."
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def put(self, instance_name):
        try:
            if not os.path.exists(HAPROXY_CONFIG):
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg "
                errmsg["ErrorDetails"] = "no such file haproxy.cfg"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
            else:
                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf'] or not data['conf']['frontend'] or not data['conf']['backend'] \
                    or not data['conf']['backend'].has_key(instance_name) or not data['conf']['frontend'].has_key(instance_name):
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input json data need frontend-backend pair format, like {'conf': {'frontend': {}, 'backend': {}}}"
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                if not data['conf']['frontend'][instance_name] or not data['conf']['backend'][instance_name]:
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input data frontend or backend {0} is empty.".format(instance_name)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                front_names = data["conf"]["frontend"].keys()
                for i in front_names:
                    if not haproxy_conf.filter(btype='Frontend', name=i):
                        errmsg["ErrorCode"] = "SLB.HAProxy.NotFound"
                        errmsg["Description"] = "not found"
                        errmsg["ErrorDetails"] = "frontend {0} is not found in haproxy.cfg, please create first".format(i)
                        self.write(errmsg)
                        self.set_status(404)
                        syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
                        return
                back_names = data["conf"]["backend"].keys()
                for i in back_names:
                    if not haproxy_conf.filter(btype='Backend', name=i):
                        errmsg["ErrorCode"] = "SLB.HAProxy.NotFound"
                        errmsg["Description"] = "not found"
                        errmsg["ErrorDetails"] = "backend {0} is not found in haproxy.cfg, please create first".format(i)
                        self.write(errmsg)
                        self.set_status(404)
                        syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
                        return

                instance_conf = HAProxyAgent.load_json(data)
                haproxy_front_obj = haproxy_conf.filter(btype='Frontend', name=instance_name)
                haproxy_conf.remove(haproxy_front_obj[0])
                haproxy_back_obj = haproxy_conf.filter(btype='Backend', name=instance_name)
                haproxy_conf.remove(haproxy_back_obj[0])
                for obj in instance_conf.children:
                    haproxy_conf.add(obj)

                # backup haproxy.cfg and test it
                if os.path.exists(HAPROXY_CONFIG_BAK):
                    os.remove(HAPROXY_CONFIG_BAK)
                if os.path.exists(HAPROXY_CONFIG_ERR):
                    os.remove(HAPROXY_CONFIG_ERR)
                os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_BAK)
                HAProxyAgent.dumpf(haproxy_conf, HAPROXY_CONFIG)
                msg, returncode = HAProxyAgent.test_haproxy_conf()
                if msg:
                    os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_ERR)
                    os.rename(HAPROXY_CONFIG_BAK, HAPROXY_CONFIG)
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy instance {0} failed, {1}".format(data, errmsg["ErrorDetails"]))
                else:
                    HAProxyAgent.reset_failed_haproxy_service()
                    HAProxyAgent.restart_haproxy_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Set haproxy instance {0} success".format(data))
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set haproxy instance failed, {0}".format(ex.message))

    def delete(self, instance_name):
        try:
            if not os.path.exists(HAPROXY_CONFIG):
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg "
                errmsg["ErrorDetails"] = "no such file haproxy.cfg"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Delete haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
            else:
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                exist_front_obj = haproxy_conf.filter(btype='Frontend', name=instance_name)
                exist_back_obj = haproxy_conf.filter(btype='Backend', name=instance_name)
                if exist_front_obj:
                    haproxy_conf.remove(exist_front_obj[0])
                    haproxy_conf.remove(exist_back_obj[0])
                if not haproxy_conf.filter(btype='Frontend'):
                    HAProxyAgent.clear_haproxy()
                    HAProxyAgent.disable_haproxy_service()
                    HAProxyAgent.stop_haproxy_service()
                else:
                    HAProxyAgent.dumpf(haproxy_conf, HAPROXY_CONFIG)
                    HAProxyAgent.restart_haproxy_service
                self.set_status(204)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Delete haproxy instance failed: {0}".format(errmsg["ErrorDetails"]))

class HAProxyFrontendsHandler(RequestHandler):
    def get(self):
        try:
            if os.path.exists(HAPROXY_CONFIG):
                frontends = []
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                front_objs = haproxy_conf.filter(btype='Frontend')
                if front_objs:
                    for k in front_objs:
                        frontends.append(k.value)
                self.write(json.dumps(frontends))
            else:
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg "
                errmsg["ErrorDetails"] = "no such file haproxy.cfg"
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Get haproxy frontends failed: {0}".format(errmsg["ErrorDetails"]))

class HAProxyFrontendHandler(RequestHandler):
    def get(self, front_name):
        try:
            if os.path.exists(HAPROXY_CONFIG):
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                front_objs = haproxy_conf.filter(btype='Frontend', name=front_name)
                if front_objs:
                    self.write(json.dumps({"conf": front_objs[0].as_dict}))
                else:
                    errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                    errmsg["Description"] = "not found"
                    errmsg["ErrorDetails"] = "not found frontend name {0} in haproxy.cfg.".format(front_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg."
                errmsg["ErrorDetails"] = "no such file haproxy.cfg."
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Get haproxy frontend failed: {0}".format(errmsg["ErrorDetails"]))

    @check_json
    def put(self, front_name):
        try:
            if not os.path.exists(HAPROXY_CONFIG):
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg "
                errmsg["ErrorDetails"] = "no such file haproxy.cfg"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
            else:
                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf'] or not data['conf']['frontend'] or not data['conf']['frontend'].has_key(front_name):
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input json data format error, like {'conf': {'frontend': {}}}, input {0}".format(data)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                if not data['conf']['frontend'][front_name]:
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input data frontend {0} is empty.".format(front_name)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                if not haproxy_conf.filter(btype='Frontend', name=front_name):
                    errmsg["ErrorCode"] = "SLB.HAProxy.NotFound"
                    errmsg["Description"] = "not found"
                    errmsg["ErrorDetails"] = "frontend {0} is not found in haproxy.cfg, please create first".format(front_name)
                    self.write(errmsg)
                    self.set_status(404)
                    syslog_error(MODULE_NAME, "Create haproxy instance failed, {0}".format(errmsg["ErrorDetails"]))
                    return

                front_conf = HAProxyAgent.load_json(data)
                haproxy_front_obj = haproxy_conf.filter(btype='Frontend', name=front_name)
                haproxy_conf.remove(haproxy_front_obj[0])
                for obj in front_conf.children:
                    haproxy_conf.add(obj)

                # backup haproxy.cfg and test it
                if os.path.exists(HAPROXY_CONFIG_BAK):
                    os.remove(HAPROXY_CONFIG_BAK)
                os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_BAK)
                HAProxyAgent.dumpf(haproxy_conf, HAPROXY_CONFIG)
                msg, returncode = HAProxyAgent.test_haproxy_conf()
                if msg and not returncode:
                    os.remove(HAPROXY_CONFIG)
                    os.rename(HAPROXY_CONFIG_BAK, HAPROXY_CONFIG)
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy frontend {0} failed, {1}".format(data, errmsg["ErrorDetails"]))
                else:
                    HAProxyAgent.reset_failed_haproxy_service()
                    HAProxyAgent.restart_haproxy_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Set haproxy frontend {0} success".format(data))
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set haproxy frontend failed, {0}".format(ex.message))

class HAProxyBackendsHandler(RequestHandler):
    def get(self):
        try:
            if os.path.exists(HAPROXY_CONFIG):
                backends = []
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                back_objs = haproxy_conf.filter(btype='Backend')
                if back_objs:
                    for k in back_objs:
                        backends.append(k.value.strip())
                self.write(json.dumps(backends))
            else:
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg "
                errmsg["ErrorDetails"] = "no such file haproxy.cfg"
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Get haproxy backends failed: {0}".format(errmsg["ErrorDetails"]))

class HAProxyBackendHandler(RequestHandler):
    def get(self, back_name):
        try:
            if os.path.exists(HAPROXY_CONFIG):
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                back_objs = haproxy_conf.filter(btype='Backend', name=back_name)
                if back_objs:
                    self.write(json.dumps({"conf": back_objs[0].as_dict}))
                else:
                    errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                    errmsg["Description"] = "not found"
                    errmsg["ErrorDetails"] = "not found backend name {0} in haproxy.cfg.".format(back_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg."
                errmsg["ErrorDetails"] = "no such file haproxy.cfg."
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Get haproxy backend failed: {0}".format(errmsg["ErrorDetails"]))

    @check_json
    def put(self, back_name):
        try:
            if not os.path.exists(HAPROXY_CONFIG):
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg "
                errmsg["ErrorDetails"] = "no such file haproxy.cfg"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set haproxy backend failed, {0}".format(errmsg["ErrorDetails"]))
            else:
                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf'] or not data['conf']['backend'] or not data['conf']['backend'].has_key(back_name):
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input json data format error, like {'conf': {'backend': {}}}, input {0}".format(data)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy backend failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                if not data['conf']['backend'][back_name]:
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input data backend {0} is empty.".format(back_name)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy backend failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                if not haproxy_conf.filter(btype='Backend', name=back_name):
                    errmsg["ErrorCode"] = "SLB.HAProxy.NotFound"
                    errmsg["Description"] = "not found"
                    errmsg["ErrorDetails"] = "backend {0} is not found in haproxy.cfg, please create first".format(back_name)
                    self.write(errmsg)
                    self.set_status(404)
                    syslog_error(MODULE_NAME, "Set haproxy backend failed, {0}".format(errmsg["ErrorDetails"]))
                    return

                back_conf = HAProxyAgent.load_json(data)
                haproxy_back_obj = haproxy_conf.filter(btype='Backend', name=back_name)
                haproxy_conf.remove(haproxy_back_obj[0])
                for obj in back_conf.children:
                    haproxy_conf.add(obj)

                # backup haproxy.cfg and test it
                if os.path.exists(HAPROXY_CONFIG_BAK):
                    os.remove(HAPROXY_CONFIG_BAK)
                if os.path.exists(HAPROXY_CONFIG_ERR):
                    os.remove(HAPROXY_CONFIG_ERR)
                os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_BAK)
                HAProxyAgent.dumpf(haproxy_conf, HAPROXY_CONFIG)
                msg, returncode = HAProxyAgent.test_haproxy_conf()
                if msg:
                    os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_ERR)
                    os.rename(HAPROXY_CONFIG_BAK, HAPROXY_CONFIG)
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy backend {0} failed, {1}".format(data, errmsg["ErrorDetails"]))
                else:
                    HAProxyAgent.reset_failed_haproxy_service()
                    HAProxyAgent.restart_haproxy_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Set haproxy backend {0} success".format(back_name))
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set haproxy backend failed, {0}".format(ex.message))

class HAProxyGlobalHandler(RequestHandler):
    def get(self):
        try:
            if os.path.exists(HAPROXY_CONFIG):
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                global_obj = haproxy_conf.filter(btype='Global')
                if global_obj:
                    self.write(json.dumps({"conf": global_obj[0].as_dict}))
                else:
                    errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                    errmsg["Description"] = "not found"
                    errmsg["ErrorDetails"] = "not found global in haproxy.cfg."
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg."
                errmsg["ErrorDetails"] = "no such file haproxy.cfg."
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Get haproxy global failed: {0}".format(errmsg["ErrorDetails"]))

    @check_json
    def put(self):
        try:
            if not os.path.exists(HAPROXY_CONFIG):
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg "
                errmsg["ErrorDetails"] = "no such file haproxy.cfg"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set haproxy global failed, {0}".format(errmsg["ErrorDetails"]))
            else:
                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf'] or not data['conf']['global']:
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input json data format error, like {'conf': {'global': {}}}, input {0}".format(data)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy global failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                if not data['conf']['global']:
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input data global is empty."
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy global failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                if not haproxy_conf.filter(btype='Global'):
                    errmsg["ErrorCode"] = "SLB.HAProxy.NotFound"
                    errmsg["Description"] = "not found"
                    errmsg["ErrorDetails"] = "global is not found in haproxy.cfg, please create first."
                    self.write(errmsg)
                    self.set_status(404)
                    syslog_error(MODULE_NAME, "Set haproxy global failed, {0}".format(errmsg["ErrorDetails"]))
                    return

                global_conf = HAProxyAgent.load_json(data)
                haproxy_global_obj = haproxy_conf.filter(btype='Global')
                haproxy_conf.remove(haproxy_global_obj[0])
                for obj in global_conf.children:
                    haproxy_conf.add(obj)

                # backup haproxy.cfg and test it
                if os.path.exists(HAPROXY_CONFIG_BAK):
                    os.remove(HAPROXY_CONFIG_BAK)
                os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_BAK)
                HAProxyAgent.dumpf(haproxy_conf, HAPROXY_CONFIG)
                msg, returncode = HAProxyAgent.test_haproxy_conf()
                if msg:
                    os.remove(HAPROXY_CONFIG)
                    os.rename(HAPROXY_CONFIG_BAK, HAPROXY_CONFIG)
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy global failed, {1}".format(errmsg["ErrorDetails"]))
                else:
                    HAProxyAgent.reset_failed_haproxy_service()
                    HAProxyAgent.restart_haproxy_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Set haproxy global success")
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set haproxy global failed, {0}".format(ex.message))

class HAProxyDefaultHandler(RequestHandler):
    def get(self):
        try:
            if os.path.exists(HAPROXY_CONFIG):
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                defaults_obj = haproxy_conf.filter(btype='Defaults')
                if defaults_obj:
                    self.write(json.dumps({"conf": defaults_obj[0].as_dict}))
                else:
                    errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                    errmsg["Description"] = "not found"
                    errmsg["ErrorDetails"] = "not found global in haproxy.cfg."
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg."
                errmsg["ErrorDetails"] = "no such file haproxy.cfg."
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Get haproxy defaults failed: {0}".format(errmsg["ErrorDetails"]))

    @check_json
    def put(self):
        try:
            if not os.path.exists(HAPROXY_CONFIG):
                errmsg["ErrorCode"] = "SLB.HAProxy.NotFound."
                errmsg["Description"] = "no such file haproxy.cfg "
                errmsg["ErrorDetails"] = "no such file haproxy.cfg"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set haproxy defaults failed, {0}".format(errmsg["ErrorDetails"]))
            else:
                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf'] or not data['conf']['defaults']:
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input json data format error, like {'conf': {'defaults': {}}}, input {0}".format(data)
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy defaults failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                if not data['conf']['defaults']:
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = "input data global is empty."
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy defaults failed, {0}".format(errmsg["ErrorDetails"]))
                    return
                
                haproxy_conf = HAProxyAgent.loadf(HAPROXY_CONFIG)
                if not haproxy_conf.filter(btype='Defaults'):
                    errmsg["ErrorCode"] = "SLB.HAProxy.NotFound"
                    errmsg["Description"] = "not found"
                    errmsg["ErrorDetails"] = "defaults is not found in haproxy.cfg, please create first."
                    self.write(errmsg)
                    self.set_status(404)
                    syslog_error(MODULE_NAME, "Set haproxy defaults failed, {0}".format(errmsg["ErrorDetails"]))
                    return

                defaults_conf = HAProxyAgent.load_json(data)
                haproxy_defaults_obj = haproxy_conf.filter(btype='Defaults')
                haproxy_conf.remove(haproxy_defaults_obj[0])
                for obj in defaults_conf.children:
                    haproxy_conf.add(obj)

                # backup haproxy.cfg and test it
                if os.path.exists(HAPROXY_CONFIG_BAK):
                    os.remove(HAPROXY_CONFIG_BAK)
                os.rename(HAPROXY_CONFIG, HAPROXY_CONFIG_BAK)
                HAProxyAgent.dumpf(haproxy_conf, HAPROXY_CONFIG)
                msg, returncode = HAProxyAgent.test_haproxy_conf()
                if msg:
                    os.remove(HAPROXY_CONFIG)
                    os.rename(HAPROXY_CONFIG_BAK, HAPROXY_CONFIG)
                    errmsg["ErrorCode"] = "SLB.HAProxy.InvalidData"
                    errmsg["Description"] = "invalid input"
                    errmsg["ErrorDetails"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set haproxy defaults failed, {1}".format(errmsg["ErrorDetails"]))
                else:
                    HAProxyAgent.reset_failed_haproxy_service()
                    HAProxyAgent.restart_haproxy_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Set haproxy defaults success")
        except Exception as ex:
            errmsg["ErrorCode"] = "SLB.HAProxy.UnknownError."
            errmsg["Description"] = "unknown error"
            errmsg["ErrorDetails"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set haproxy defaults failed, {0}".format(ex.message))
