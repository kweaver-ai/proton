#!/usr/bin/env python
# -*- coding: utf-8 -*-
# @Time    : 2021/4/2 9:56
# @Author  : mo.kang<mo.kang@eisoo.com>
# @Site    :
# @File    : router.py
# @Software: PyCharm

from tornado.web import StaticFileHandler

from src.handler.ecms_handler import NetArpHandler, NetInfNameHandler, NetIpsHandler, NetNicIfaddrHandler, \
    NetNicIpaddrHandler, NetNicsHandler, FirewalldDefaultZoneHandler, FirewalldFirewallHandler, \
    FirewalldHandler, FirewalldRichRulesHandler, FirewalldServicesHandler, FirewalldSourcesHandler, \
    FirewalldTargetsHandler, DirectoryHandler, CodeHandler, SysctlHandler, ChronyRoleHandler, ChronyHandler, \
    ChronyDiffHandler, ChronyServerHandler, TLSHandler, ServiceHandler, SysInfoHandler

from src.handler.v1alpha1 import FileHandler as V1Alpha1FileHandler
from src.handler.v1alpha1 import FileMovementHandler as V1Alpha1FileMovementHandler
from src.handler.v1alpha1 import ExecutionHandler as V1Alpha1ExecutionHandler
from src.handler.nginx_handler import NginxInstanceHandler, NginxHttpHandler, NginxHttpServerHandler
from src.handler.keepalived_handler_v2 import KeepalivedInstanceHandler, KeepalivedHAHandlerV2, \
    KeepalivedHAInstanceHandler
from src.handler.haproxy_handler import HAProxyConfigHandler

ECMS_ROUTER = [
    # NetAgent
    (r"/api/ecms/v1/net/interface-names", NetInfNameHandler),
    (r"/api/ecms/v1/net/arp/(.+)", NetArpHandler),
    (r"/api/ecms/v1/net/ips", NetIpsHandler),
    (r"/api/ecms/v1/net/nic/ifaddr", NetNicIfaddrHandler),
    (r"/api/ecms/v1/net/nic/(.+)/ipaddr", NetNicIpaddrHandler),
    (r"/api/ecms/v1/net/nics", NetNicsHandler),
    # FirewallAgent
    (r"/api/ecms/v1/firewalld/default-zones", FirewalldDefaultZoneHandler),
    (r"/api/ecms/v1/firewalld/firewall", FirewalldFirewallHandler),
    (r"/api/ecms/v1/firewalld", FirewalldHandler),
    (r"/api/ecms/v1/firewalld/rich-rules", FirewalldRichRulesHandler),
    (r"/api/ecms/v1/firewalld/services", FirewalldServicesHandler),
    (r"/api/ecms/v1/firewalld/sources", FirewalldSourcesHandler),
    (r"/api/ecms/v1/firewalld/targets", FirewalldTargetsHandler),
    # FileAgent
    (r"/api/ecms/v1/file", DirectoryHandler),
    (r"/api/ecms/v1/file/tls", TLSHandler),
    # SystemAgent
    (r"/api/ecms/v1/system/machine-code", CodeHandler),
    (r"/api/ecms/v1/system/sysctl", SysctlHandler),
    (r"/api/ecms/v1/system/service", ServiceHandler),
    (r"/api/ecms/v1/system/sysinfo", SysInfoHandler),
    # ChronyAgent
    (r"/api/ecms/v1/chrony/role", ChronyRoleHandler),
    (r"/api/ecms/v1/chrony/diff", ChronyDiffHandler),
    (r"/api/ecms/v1/chrony/server", ChronyServerHandler),
    (r"/api/ecms/v1/chrony/chrony", ChronyHandler),
    # Proton SLB compatibility
    (r"/api/slb/v1/nginx/nginx", NginxInstanceHandler),
    (r"/api/slb/v1/nginx/http", NginxHttpHandler),
    (r"/api/slb/v1/nginx/http/(.+)", NginxHttpServerHandler),
    (r"/api/slb/v2/keepalived/keepalived", KeepalivedInstanceHandler),
    (r"/api/slb/v2/keepalived/ha", KeepalivedHAHandlerV2),
    (r"/api/slb/v2/keepalived/ha/(.+)", KeepalivedHAInstanceHandler),
    (r"/api/slb/v1/haproxy/haproxy", HAProxyConfigHandler),
    # Execution
    (r"/api/ecms/v1alpha1/exec", V1Alpha1ExecutionHandler),
    # Files
    (r"/api/ecms/v1alpha1/files/(.+)/movement", V1Alpha1FileMovementHandler),
    (r"/api/ecms/v1alpha1/files/(.+)", V1Alpha1FileHandler),
]
