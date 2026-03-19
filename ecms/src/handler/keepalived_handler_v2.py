#!/usr/bin/env python
# -*- coding:utf-8 -*-

"""
Copyright (C) EISOO Systems, Inc - All Rights Reserved
  Unauthorized copying of this file, via any medium is strictly prohibited
  Proprietary and confidential
  Written by Kelu Tang <tang.kelu@eisoo.com>, February 2019
"""
import os
import sys
import uuid
import json
import shutil
from tornado.web import RequestHandler

CURR_SCRIPT_PATH = os.path.dirname(os.path.abspath(sys.argv[0]))
sys.path.append(os.path.dirname(CURR_SCRIPT_PATH))

from src.modules.pydeps.logger import syslog, syslog_error
from src.modules.keepalived_agent_v2 import KeepalivedAgent, Global_defs, Authentication, Key
from src.handler.nginx_handler import check_json

KEEPALIVED_CONFIG = "/etc/keepalived/keepalived.conf"
KEEPALIVED_CONFIG_BAK = "/etc/keepalived/keepalived.conf.tmpfile"
CLUSTER_MASTER = "/etc/slb/scripts/entering_master.py"
CLUSTER_BACKUP = "/etc/slb/scripts/entering_backup.py"
INSTANCE_SCRIPT_PATH = "/etc/slb/scripts"
MODULE_NAME = "Proton_SLB_Manager"
errmsg = {
    "code": "",
    "message": "",
    "cause": "",
    "detail": ""
}


class KeepalivedStatusHandlerV2(RequestHandler):
    def get(self):
        try:
            status = KeepalivedAgent.get_keepalived_status()
            self.write(json.dumps({"status": status}))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)


class KeepalivedServiceStopHandlerV2(RequestHandler):
    def put(self):
        try:
            KeepalivedAgent.stop_keepalived_service()
            self.set_status(204)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)


class KeepalivedServiceStartHandlerV2(RequestHandler):
    def put(self):
        try:
            KeepalivedAgent.start_keepalived_service()
            self.set_status(204)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)


class KeepalivedServiceReloadHandlerV2(RequestHandler):
    def put(self):
        try:
            if self.request.body:
                data = json.loads(self.request.body)
                KeepalivedAgent.reload_keepalived_service(data["is_vrrp_changed"])
            else:
                KeepalivedAgent.reload_keepalived_service(is_vrrp_changed=False)
            self.set_status(204)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)


class KeepalivedServiceEnableHandlerV2(RequestHandler):
    def put(self):
        try:
            KeepalivedAgent.enable_keepalived_service()
            self.set_status(204)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)


class KeepalivedServiceDisableHandlerV2(RequestHandler):
    def put(self):
        try:
            KeepalivedAgent.disable_keepalived_service()
            self.set_status(204)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)


class KeepalivedInstanceHandler(RequestHandler):
    def get(self):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                self.write(json.dumps(keepalived_conf.as_dict))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance not found"
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
            if os.path.getsize(KEEPALIVED_CONFIG):
                errmsg["code"] = "409010000"
                errmsg["message"] = "conflict"
                errmsg["cause"] = "keepalived instance already exists"
                self.write(errmsg)
                self.set_status(409)
            else:
                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf']:
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = "need conf section"
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create keepalived instance failed, {0}".format(errmsg["cause"]))
                    return
                keepalived_conf = KeepalivedAgent.load_json(data)
                KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                msg = KeepalivedAgent.test_keepalived_conf()
                if msg:
                    KeepalivedAgent.clear_keepalived()
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create keepalived instance failed, {0}".format(msg))
                else:
                    KeepalivedAgent.enable_keepalived_service()
                    KeepalivedAgent.restart_keepalived_service()
                    self.set_status(201)
                    syslog(MODULE_NAME, "Create keepalived instance success")
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create keepalived instance failed, {0}".format(ex.message))

    @check_json
    def put(self):
        try:
            if not os.path.exists(KEEPALIVED_CONFIG):
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance not found"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Set keepalived instance failed, {0}".format(errmsg["cause"]))
            else:
                # convert unicode to utf8
                data = eval(self.request.body)
                if not data.has_key('conf') or not data['conf']:
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = "need conf section"
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set keepalived instance failed, {0}".format(errmsg["cause"]))
                    return

                keepalived_conf = KeepalivedAgent.load_json(data)

                # backup keepalived.conf and test it
                if os.path.exists(KEEPALIVED_CONFIG_BAK):
                    os.remove(KEEPALIVED_CONFIG_BAK)
                os.rename(KEEPALIVED_CONFIG, KEEPALIVED_CONFIG_BAK)
                KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                msg = KeepalivedAgent.test_keepalived_conf()
                if msg:
                    os.remove(KEEPALIVED_CONFIG)
                    os.rename(KEEPALIVED_CONFIG_BAK, KEEPALIVED_CONFIG)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Set keepalived instance failed, {0}".format(errmsg["cause"]))
                else:
                    KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                    if KeepalivedAgent.is_failed_keepalived_service():
                        KeepalivedAgent.reset_failed_keepalived_service()
                    if KeepalivedAgent.get_keepalived_status():
                        KeepalivedAgent.reload_keepalived_service()
                    else:
                        KeepalivedAgent.restart_keepalived_service()
                    self.set_status(204)
                    syslog(MODULE_NAME, "Set keepalived instance success")
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Set keepalived instance failed, {0}".format(ex.message))

    def delete(self):
        try:
            KeepalivedAgent.clear_keepalived()
            KeepalivedAgent.disable_keepalived_service()
            KeepalivedAgent.stop_keepalived_service()
            self.set_status(204)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Delete keepalived instance failed: {0}".format(errmsg["cause"]))


class KeepalivedHAHandlerV2(RequestHandler):
    def get(self):
        try:
            instance = []
            keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
            inst_obj = keepalived_conf.filter(btype='Vrrp_instance')
            if inst_obj:
                for k in inst_obj:
                    instance.append(k.value)
            self.write(json.dumps(instance))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self):
        try:
            if not os.path.exists(KEEPALIVED_CONFIG):
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "{0} not found".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Create keepalived HA instance failed, {0}".format(errmsg["cause"]))
            else:
                data = eval(self.request.body)
                instance_conf = KeepalivedAgent.load_json(data)
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                if not keepalived_conf.children:
                    uid = uuid.uuid1()
                    global_obj = Global_defs()
                    global_obj.add(Key("router_id", str(uid)))
                    global_obj.add(Key("vrrp_garp_master_refresh", '30'))
                    keepalived_conf.add(global_obj)
                for k in instance_conf.filter(btype='Vrrp_instance'):
                    exist_obj = keepalived_conf.filter(btype='Vrrp_instance', name=k.value)
                    if exist_obj:
                        errmsg["code"] = "404010000"
                        errmsg["message"] = "conflict"
                        errmsg["cause"] = "keepalived ha instance {0} is already exist".format(k.value)
                        self.write(errmsg)
                        self.set_status(409)
                        syslog_error(MODULE_NAME, "Create keepalived HA instance failed, {0}".format(errmsg["cause"]))
                        return
                    else:
                        INST_SCRIPT_DIR = os.path.join(INSTANCE_SCRIPT_PATH, k.value)
                        INST_MASTER_SCRIPT_PATH = os.path.join(INST_SCRIPT_DIR, os.path.basename(CLUSTER_MASTER))
                        INST_BACKUP_SCRIPT_PATH = os.path.join(INST_SCRIPT_DIR, os.path.basename(CLUSTER_BACKUP))
                        if not os.path.isdir(INST_SCRIPT_DIR):
                            os.mkdir(INST_SCRIPT_DIR)
                        shutil.copy(CLUSTER_MASTER, INST_MASTER_SCRIPT_PATH)
                        shutil.copy(CLUSTER_BACKUP, INST_BACKUP_SCRIPT_PATH)
                        k.add(Key("state", "BACKUP"))
                        k.add(Key("advert_int", "1"))
                        k.add(Key("nopreempt", ""))
                        k.add(Key("notify_master", INST_MASTER_SCRIPT_PATH))
                        k.add(Key("notify_backup", INST_BACKUP_SCRIPT_PATH))

                        keepalived_conf.add(k)

                # backup keepalived.conf and test it
                if os.path.exists(KEEPALIVED_CONFIG_BAK):
                    os.remove(KEEPALIVED_CONFIG_BAK)
                os.rename(KEEPALIVED_CONFIG, KEEPALIVED_CONFIG_BAK)
                KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                msg = KeepalivedAgent.test_keepalived_conf()
                if msg:
                    os.remove(KEEPALIVED_CONFIG)
                    os.rename(KEEPALIVED_CONFIG_BAK, KEEPALIVED_CONFIG)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create keepalived HA instance failed, {0}".format(errmsg["cause"]))
                else:
                    if data.has_key('reload') and data["reload"] == "false":
                        syslog(MODULE_NAME, "Create keepalived HA instance success, no need reload")
                    else:
                        if KeepalivedAgent.get_keepalived_status():
                            KeepalivedAgent.reload_keepalived_service()
                        else:
                            KeepalivedAgent.restart_keepalived_service()
                        syslog(MODULE_NAME, "Create keepalived HA instance success")
                    self.set_status(201)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create keepalived HA instance failed: {0}".format(errmsg["cause"]))


class KeepalivedHAInstanceHandler(RequestHandler):
    def get(self, instance_name):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                inst_obj = keepalived_conf.filter(btype='Vrrp_instance', name=instance_name)
                if inst_obj:
                    self.write(json.dumps(inst_obj[0].as_dict))
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived HA instance {0} not found".format(instance_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance {0} not found or empty".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def put(self, instance_name):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                inst_obj = keepalived_conf.filter(btype='Vrrp_instance', name=instance_name)
                if inst_obj:
                    keepalived_conf.remove(inst_obj[0])
                    data = eval(self.request.body)
                    new_obj = KeepalivedAgent.load_json(data)
                    for k in new_obj.filter(btype='Vrrp_instance'):
                        keepalived_conf.add(k)

                    # backup keepalived.conf and test it
                    if os.path.exists(KEEPALIVED_CONFIG_BAK):
                        os.remove(KEEPALIVED_CONFIG_BAK)
                    os.rename(KEEPALIVED_CONFIG, KEEPALIVED_CONFIG_BAK)
                    KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                    msg = KeepalivedAgent.test_keepalived_conf()
                    if msg:
                        os.remove(KEEPALIVED_CONFIG)
                        os.rename(KEEPALIVED_CONFIG_BAK, KEEPALIVED_CONFIG)
                        errmsg["code"] = "400010000"
                        errmsg["message"] = "invalid input"
                        errmsg["cause"] = msg
                        self.write(errmsg)
                        self.set_status(400)
                        syslog_error(MODULE_NAME, "Set keepalived HA instance {0} failed, {1}".format(instance_name,
                                                                                                      errmsg["cause"]))
                    else:
                        if data.has_key('reload') and data["reload"] == "false":
                            syslog(MODULE_NAME,
                                   "Set keepalived HA instance {0} success, no need reload".format(instance_name))
                        else:
                            if KeepalivedAgent.get_keepalived_status():
                                KeepalivedAgent.reload_keepalived_service()
                            else:
                                KeepalivedAgent.restart_keepalived_service()
                            syslog(MODULE_NAME, "Set keepalived HA instance {0} success".format(instance_name))
                        self.set_status(204)
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived HA instance {0} not found or empty".format(instance_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance {0} not found or empty".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Set keepalived HA instance {0} failed: {1}".format(instance_name, errmsg["cause"]))

    @check_json
    def delete(self, instance_name):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                inst_obj = keepalived_conf.filter(btype='Vrrp_instance', name=instance_name)
                if inst_obj:
                    keepalived_conf.remove(inst_obj[0])
                    KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                    if self.request.body:
                        data = eval(self.request.body)
                        if data.has_key('reload') and data["reload"] == "false":
                            syslog(MODULE_NAME,
                                   "Delete keepalived HA instance {0} success, no need reload".format(instance_name))
                        else:
                            if KeepalivedAgent.get_keepalived_status():
                                KeepalivedAgent.reload_keepalived_service()
                            else:
                                KeepalivedAgent.restart_keepalived_service()
                    else:
                        if KeepalivedAgent.get_keepalived_status():
                            KeepalivedAgent.reload_keepalived_service()
                        else:
                            KeepalivedAgent.restart_keepalived_service()
                    syslog(MODULE_NAME, "Delete keepalived HA instance {0} success".format(instance_name))
                else:
                    syslog(MODULE_NAME, "Delete keepalived HA instance: not found instance {0}, nothing to do.".format(
                        instance_name))
                INST_SCRIPT_DIR = os.path.join(INSTANCE_SCRIPT_PATH, instance_name)
                if os.path.isdir(INST_SCRIPT_DIR):
                    shutil.rmtree(INST_SCRIPT_DIR)
                self.set_status(204)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived config {0} not exist".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Delete keepalived HA instance {0} failed: {1}".format(instance_name, errmsg["cause"]))


class KeepalivedVrrpScriptHandler(RequestHandler):
    def get(self):
        try:
            vrrp_script = []
            keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
            vrrp_script_obj = keepalived_conf.filter(btype='Vrrp_script')
            if vrrp_script_obj:
                for k in vrrp_script_obj:
                    vrrp_script.append(k.value)
            self.write(json.dumps(vrrp_script))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self):
        try:
            if not os.path.exists(KEEPALIVED_CONFIG):
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance not found"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Create keepalived vrrp_script failed, {0}".format(errmsg["cause"]))
            else:
                data = eval(self.request.body)
                vrrp_script_conf = KeepalivedAgent.load_json(data)
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                if not keepalived_conf.children:
                    uid = uuid.uuid1()
                    global_obj = Global_defs()
                    global_obj.add(Key("router_id", str(uid)))
                    global_obj.add(Key("vrrp_garp_master_refresh", '30'))
                    keepalived_conf.add(global_obj)
                for k in vrrp_script_conf.filter(btype='Vrrp_script'):
                    exist_obj = keepalived_conf.filter(btype='Vrrp_script', name=k.value)
                    if exist_obj:
                        errmsg["code"] = "404010000"
                        errmsg["message"] = "conflict"
                        errmsg["cause"] = "keepalived vrrp_script {0} is already exist".format(k.value)
                        self.write(errmsg)
                        self.set_status(409)
                        syslog_error(MODULE_NAME, "Create keepalived vrrp_script failed, {0}".format(errmsg["cause"]))
                        return
                    else:
                        keepalived_conf.add(k)

                # backup keepalived.conf and test it
                if os.path.exists(KEEPALIVED_CONFIG_BAK):
                    os.remove(KEEPALIVED_CONFIG_BAK)
                os.rename(KEEPALIVED_CONFIG, KEEPALIVED_CONFIG_BAK)
                KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                msg = KeepalivedAgent.test_keepalived_conf()
                if msg:
                    os.remove(KEEPALIVED_CONFIG)
                    os.rename(KEEPALIVED_CONFIG_BAK, KEEPALIVED_CONFIG)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create keepalived vrrp_script failed, {0}".format(errmsg["cause"]))
                else:
                    syslog(MODULE_NAME, "Create keepalived vrrp_script success")
                    self.set_status(201)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create keepalived vrrp_script failed: {0}".format(errmsg["cause"]))


class KeepalivedVrrpScriptInstancetHandler(RequestHandler):
    def get(self, vrrp_script_name):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                vrrp_script_obj = keepalived_conf.filter(btype='Vrrp_script', name=vrrp_script_name)
                if vrrp_script_obj:
                    self.write(json.dumps(vrrp_script_obj[0].as_dict))
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived vrrp_script {0} not found".format(vrrp_script_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance {0} not found or empty".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def put(self, vrrp_script_name):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                vrrp_script_obj = keepalived_conf.filter(btype='Vrrp_script', name=vrrp_script_name)
                if vrrp_script_obj:
                    keepalived_conf.remove(vrrp_script_obj[0])
                    data = eval(self.request.body)
                    new_obj = KeepalivedAgent.load_json(data)
                    for k in new_obj.filter(btype='Vrrp_script'):
                        keepalived_conf.add(k)

                    # backup keepalived.conf and test it
                    if os.path.exists(KEEPALIVED_CONFIG_BAK):
                        os.remove(KEEPALIVED_CONFIG_BAK)
                    os.rename(KEEPALIVED_CONFIG, KEEPALIVED_CONFIG_BAK)
                    KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                    msg = KeepalivedAgent.test_keepalived_conf()
                    if msg:
                        os.remove(KEEPALIVED_CONFIG)
                        os.rename(KEEPALIVED_CONFIG_BAK, KEEPALIVED_CONFIG)
                        errmsg["code"] = "400010000"
                        errmsg["message"] = "invalid input"
                        errmsg["cause"] = msg
                        self.write(errmsg)
                        self.set_status(400)
                        syslog_error(MODULE_NAME, "Set keepalived vrrp_script {0} failed, {1}".format(vrrp_script_name,
                                                                                                      errmsg["cause"]))
                    else:
                        self.set_status(204)
                        syslog(MODULE_NAME, "Set keepalived vrrp_script {0} success".format(vrrp_script_name))
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived vrrp_script {0} not found or empty".format(vrrp_script_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived config {0} not exist".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Set keepalived vrrp_script {0} failed: {1}".format(vrrp_script_name, errmsg["cause"]))

    def delete(self, vrrp_script_name):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                vrrp_script_obj = keepalived_conf.filter(btype='Vrrp_script', name=vrrp_script_name)
                if vrrp_script_obj:
                    keepalived_conf.remove(vrrp_script_obj[0])
                    KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                    KeepalivedAgent.reload_keepalived_service()
                self.set_status(204)
                syslog(MODULE_NAME, "Delete keepalived vrrp_script {0} success".format(vrrp_script_name))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived conf {0} not exist".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Delete keepalived vrrp_script {0} failed: {1}".format(vrrp_script_name, errmsg["cause"]))


class KeepalivedVSHandler(RequestHandler):
    def get(self):
        try:
            vs = []
            keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
            vs_obj = keepalived_conf.filter(btype='Virtual_server')
            if vs_obj:
                for k in vs_obj:
                    vs.append(k.value)
            self.write(json.dumps(vs))
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self):
        try:
            if not os.path.exists(KEEPALIVED_CONFIG) or not os.path.getsize(KEEPALIVED_CONFIG):
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance not found"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Create keepalived virtual server failed, {0}".format(errmsg["cause"]))
            else:
                data = eval(self.request.body)
                vs_conf = KeepalivedAgent.load_json(data)
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                for k in vs_conf.filter(btype='Virtual_server'):
                    exist_obj = keepalived_conf.filter(btype='Virtual_server', name=k.value)
                    if exist_obj:
                        errmsg["code"] = "404010000"
                        errmsg["message"] = "conflict"
                        errmsg["cause"] = "keepalived virtual server {0} is already exist".format(k.value)
                        self.write(errmsg)
                        self.set_status(409)
                        syslog_error(MODULE_NAME,
                                     "Create keepalived virtual server failed, {0}".format(errmsg["cause"]))
                        return
                    else:
                        keepalived_conf.add(k)

                # backup keepalived.conf and test it
                if os.path.exists(KEEPALIVED_CONFIG_BAK):
                    os.remove(KEEPALIVED_CONFIG_BAK)
                os.rename(KEEPALIVED_CONFIG, KEEPALIVED_CONFIG_BAK)
                KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                msg = KeepalivedAgent.test_keepalived_conf()
                if msg:
                    os.remove(KEEPALIVED_CONFIG)
                    os.rename(KEEPALIVED_CONFIG_BAK, KEEPALIVED_CONFIG)
                    errmsg["code"] = "400010000"
                    errmsg["message"] = "invalid input"
                    errmsg["cause"] = msg
                    self.write(errmsg)
                    self.set_status(400)
                    syslog_error(MODULE_NAME, "Create keepalived virtual server failed, {0}".format(errmsg["cause"]))
                else:
                    KeepalivedAgent.reload_keepalived_service()
                    syslog(MODULE_NAME, "Create keepalived virtual server success")
                    self.set_status(201)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create keepalived virtual server failed: {0}".format(errmsg["cause"]))


class KeepalivedVSInstanceHandler(RequestHandler):
    def get(self, vip, port):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                virtual_name = " ".join([vip, port])
                vs_obj = keepalived_conf.filter(btype='Virtual_server', name=virtual_name)
                if vs_obj:
                    self.write(json.dumps(vs_obj[0].as_dict))
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived virtual server {0} not found".format(virtual_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance {0} not found or empty".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def put(self, vip, port):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                virtual_name = " ".join([vip, port])
                vs_obj = keepalived_conf.filter(btype='Virtual_server', name=virtual_name)
                if vs_obj:
                    keepalived_conf.remove(vs_obj[0])
                    data = eval(self.request.body)
                    new_obj = KeepalivedAgent.load_json(data)
                    for k in new_obj.filter(btype='Virtual_server'):
                        keepalived_conf.add(k)

                    # backup keepalived.conf and test it
                    if os.path.exists(KEEPALIVED_CONFIG_BAK):
                        os.remove(KEEPALIVED_CONFIG_BAK)
                    os.rename(KEEPALIVED_CONFIG, KEEPALIVED_CONFIG_BAK)
                    KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                    msg = KeepalivedAgent.test_keepalived_conf()
                    if msg:
                        os.remove(KEEPALIVED_CONFIG)
                        os.rename(KEEPALIVED_CONFIG_BAK, KEEPALIVED_CONFIG)
                        errmsg["code"] = "400010000"
                        errmsg["message"] = "invalid input"
                        errmsg["cause"] = msg
                        self.write(errmsg)
                        self.set_status(400)
                        syslog_error(MODULE_NAME, "Set keepalived virtual server {0} failed, {1}".format(virtual_name,
                                                                                                         errmsg[
                                                                                                             "cause"]))
                    else:
                        KeepalivedAgent.reload_keepalived_service()
                        self.set_status(204)
                        syslog(MODULE_NAME, "Set keepalived virtual server {0} success".format(virtual_name))
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived virtual server {0} not found".format(virtual_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance {0} not found or empty".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Set keepalived virtual server {0} failed: {1}".format(virtual_name, errmsg["cause"]))

    def delete(self, vip, port):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                virtual_name = " ".join([vip, port])
                vs_obj = keepalived_conf.filter(btype='Virtual_server', name=virtual_name)
                if vs_obj:
                    keepalived_conf.remove(vs_obj[0])
                    KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                    KeepalivedAgent.reload_keepalived_service()
                self.set_status(204)
                syslog(MODULE_NAME, "Delete keepalived virtual server {0} success".format(virtual_name))
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance {0} not found or empty".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Delete keepalived virtual server {0} failed: {1}".format(virtual_name, errmsg["cause"]))


class KeepalivedVSRSHandler(RequestHandler):
    def get(self, vip, port):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                rs = []
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                virtual_name = " ".join([vip, port])
                vs_obj = keepalived_conf.filter(btype='Virtual_server', name=virtual_name)
                if vs_obj:
                    rs_obj = vs_obj[0].filter(btype='Real_server')
                    if rs_obj:
                        for k in rs_obj:
                            rs.append(k.value)
                    self.write(json.dumps(rs))
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived virtual server {0} not found".format(virtual_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance {0} not found or empty".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def post(self, vip, port):
        try:
            if not os.path.exists(KEEPALIVED_CONFIG) or not os.path.getsize(KEEPALIVED_CONFIG):
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance not found"
                self.write(errmsg)
                self.set_status(404)
                syslog_error(MODULE_NAME, "Create keepalived real server failed, {0}".format(errmsg["cause"]))
            else:
                data = eval(self.request.body)
                virtual_name = " ".join([vip, port])
                rs_conf = KeepalivedAgent.load_json(data)
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                vs_obj = keepalived_conf.filter(btype='Virtual_server', name=virtual_name)
                if vs_obj:
                    for k in rs_conf.filter(btype='Virtual_server', name=virtual_name)[0].filter(btype='Real_server'):
                        exist_obj = vs_obj[0].filter(btype='Real_server', name=k.value)
                        if exist_obj:
                            errmsg["code"] = "404010000"
                            errmsg["message"] = "conflict"
                            errmsg["cause"] = "keepalived virtual server {0} real server {1} is already exist".format(
                                virtual_name, k.value)
                            self.write(errmsg)
                            self.set_status(409)
                            syslog_error(MODULE_NAME,
                                         "Create keepalived real server failed, {0}".format(errmsg["cause"]))
                            return
                        else:
                            vs_obj[0].add(k)

                    # backup keepalived.conf and test it
                    if os.path.exists(KEEPALIVED_CONFIG_BAK):
                        os.remove(KEEPALIVED_CONFIG_BAK)
                    os.rename(KEEPALIVED_CONFIG, KEEPALIVED_CONFIG_BAK)
                    KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                    msg = KeepalivedAgent.test_keepalived_conf()
                    if msg:
                        os.remove(KEEPALIVED_CONFIG)
                        os.rename(KEEPALIVED_CONFIG_BAK, KEEPALIVED_CONFIG)
                        errmsg["code"] = "400010000"
                        errmsg["message"] = "invalid input"
                        errmsg["cause"] = msg
                        self.write(errmsg)
                        self.set_status(400)
                        syslog_error(MODULE_NAME, "Create keepalived real server failed, {0}".format(errmsg["cause"]))
                    else:
                        KeepalivedAgent.reload_keepalived_service()
                        syslog(MODULE_NAME, "Create keepalived real server success")
                        self.set_status(201)
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived virtual server {0} not found".format(virtual_name)
                    self.write(errmsg)
                    self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME, "Create keepalived virtual server failed: {0}".format(errmsg["cause"]))


class KeepalivedVSRSInstanceHandler(RequestHandler):
    def get(self, vip, port, rip):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                virtual_name = " ".join([vip, port])
                real_name = " ".join([rip, port])
                vs_obj = keepalived_conf.filter(btype='Virtual_server', name=virtual_name)
                if vs_obj:
                    rs_obj = vs_obj[0].filter(btype='Real_server', name=real_name)
                    if rs_obj:
                        self.write(json.dumps(rs_obj[0].as_dict))
                    else:
                        errmsg["code"] = "404010000"
                        errmsg["message"] = "not found"
                        errmsg["cause"] = "keepalived virtual server {0} real server {1} not found".format(virtual_name,
                                                                                                           real_name)
                        self.write(errmsg)
                        self.set_status(404)
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived virtual server {0} not found".format(virtual_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance {0} not found or empty".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)

    @check_json
    def put(self, vip, port, rip):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                virtual_name = " ".join([vip, port])
                real_name = " ".join([rip, port])
                vs_obj = keepalived_conf.filter(btype='Virtual_server', name=virtual_name)
                if vs_obj:
                    rs_obj = vs_obj[0].filter(btype='Real_server', name=real_name)
                    if rs_obj:
                        vs_obj[0].remove(rs_obj[0])
                        data = eval(self.request.body)
                        new_obj = KeepalivedAgent.load_json(data)
                        vs_new_obj = new_obj.filter(btype='Virtual_server', name=virtual_name)
                        for k in vs_new_obj[0].filter(btype='Real_server'):
                            vs_obj[0].add(k)

                        # backup keepalived.conf and test it
                        if os.path.exists(KEEPALIVED_CONFIG_BAK):
                            os.remove(KEEPALIVED_CONFIG_BAK)
                        os.rename(KEEPALIVED_CONFIG, KEEPALIVED_CONFIG_BAK)
                        KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                        msg = KeepalivedAgent.test_keepalived_conf()
                        if msg:
                            os.remove(KEEPALIVED_CONFIG)
                            os.rename(KEEPALIVED_CONFIG_BAK, KEEPALIVED_CONFIG)
                            errmsg["code"] = "400010000"
                            errmsg["message"] = "invalid input"
                            errmsg["cause"] = msg
                            self.write(errmsg)
                            self.set_status(400)
                            syslog_error(MODULE_NAME,
                                         "Set keepalived virtual server {0} real server {1} failed, {2}".format(
                                             virtual_name, real_name, errmsg["cause"]))
                        else:
                            KeepalivedAgent.reload_keepalived_service()
                            self.set_status(204)
                            syslog(MODULE_NAME,
                                   "Set keepalived virtual server {0} real server {1} success".format(virtual_name,
                                                                                                      real_name))
                    else:
                        errmsg["code"] = "404010000"
                        errmsg["message"] = "not found"
                        errmsg["cause"] = "keepalived virtual server {0} real server {1} not found".format(virtual_name,
                                                                                                           real_name)
                        self.write(errmsg)
                        self.set_status(404)
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived virtual server {0} not found".format(virtual_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance {0} not found or empty".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Set keepalived virtual server {0} real server {1} failed: {2}".format(virtual_name, real_name,
                                                                                                errmsg["cause"]))

    def delete(self, vip, port, rip):
        try:
            if os.path.exists(KEEPALIVED_CONFIG):
                keepalived_conf = KeepalivedAgent.loadf(KEEPALIVED_CONFIG)
                virtual_name = " ".join([vip, port])
                real_name = " ".join([rip, port])
                vs_obj = keepalived_conf.filter(btype='Virtual_server', name=virtual_name)
                if vs_obj:
                    rs_obj = vs_obj[0].filter(btype='Real_server', name=real_name)
                    if rs_obj:
                        vs_obj[0].remove(rs_obj[0])
                        KeepalivedAgent.dumpf(keepalived_conf, KEEPALIVED_CONFIG)
                        KeepalivedAgent.reload_keepalived_service()
                    self.set_status(204)
                    syslog(MODULE_NAME,
                           "Delete keepalived virtual server {0} real server {1} success".format(virtual_name,
                                                                                                 real_name))
                else:
                    errmsg["code"] = "404010000"
                    errmsg["message"] = "not found"
                    errmsg["cause"] = "keepalived virtual server {0} not found".format(virtual_name)
                    self.write(errmsg)
                    self.set_status(404)
            else:
                errmsg["code"] = "404010000"
                errmsg["message"] = "not found"
                errmsg["cause"] = "keepalived instance {0} not found or empty".format(KEEPALIVED_CONFIG)
                self.write(errmsg)
                self.set_status(404)
        except Exception as ex:
            errmsg["code"] = "500010000"
            errmsg["message"] = "unknown error"
            errmsg["cause"] = ex.message
            self.write(errmsg)
            self.set_status(500)
            syslog_error(MODULE_NAME,
                         "Delete keepalived virtual server {0} real server {1} failed: {2}".format(virtual_name,
                                                                                                   real_name,
                                                                                                   errmsg["cause"]))
