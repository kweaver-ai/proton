#!/bin/bash
# This script checks if a database container is healthy based on cluster type.
# The purpose of this script is to make Docker capable of monitoring different
# database cluster type properly.
# This script will just return exit 0 (OK) or 1 (NOT OK).

MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3306"
MYSQL_USERNAME=${MYSQL_ROOT_USER}
MYSQL_PASSWORD=${MYSQL_ROOT_PASSWORD}
MYSQL_OPTS="-N -q -A --connect-timeout=10 --host=$MYSQL_HOST --port=$MYSQL_PORT --user=$MYSQL_USERNAME --password=$MYSQL_PASSWORD"


MYSQL_BIN='/usr/bin/mariadb'
CHECK_SQL="select 12345"
CHECK_WSREP="show global variables like 'wsrep_on'"
CHECK_QUERY="show global status where variable_name='wsrep_ready'"
CHECK_QUERY2="show global variables where variable_name='read_only'"
CHECK_QUERY3="show global status where variable_name='wsrep_local_state_comment'"
READINESS=0
LIVENESS=0

CHECK_FLAG=""
# Kubernetes' readiness & liveness flag
if [ ! -z $1 ]; then
	CHECK_FLAG=$1
	[ $1 == "--readiness" ] && READINESS=1
	[ $1 == "--liveness" ] && LIVENESS=1
fi

# logging functions
mysql_log() {
	local type="$1"; shift
	printf '%s [%s] [Healthcheck] [%s]: %s\n' "$(date --rfc-3339=seconds)" "$type" "$CHECK_FLAG" "$*" >> /var/lib/mysql/healthcheck.log
}

return_ok()
{
	exit 0
}
return_fail()
{
	exit 1
}

mysql_log "Note" "check begin"


#If SST, return OK
if [ $LIVENESS -eq 1 ] && [ `ps -ef | grep wsrep_sst_mariabackup | grep -v grep | wc -l` != 0 ]; then
	mysql_log "Note" "SST"
	mysql_log "Note" "Check Liveness Success"
	return_ok
fi

#If during waiting, return OK
if [ $LIVENESS -eq 1 ] && [ `ps -ef | grep entrypoint.sh | grep -v grep | wc -l` != 0 ]; then
	mysql_log "Note" "Waiting for entrypoint.sh finish"
	mysql_log "Note" "Check Liveness Success"
	return_ok
fi

# if not connect, return fail
$MYSQL_BIN $MYSQL_OPTS -e "$CHECK_WSREP" || return_fail

wsrep_on=$($MYSQL_BIN $MYSQL_OPTS -e "$CHECK_WSREP;" 2>/dev/null | awk '{print $2;}')
mysql_log "Note" "WSREP_ON IS '$wsrep_on'"

if [ "$wsrep_on" = "OFF" ]; then
	mysql_log "Note" "WSREP_ON IS OFF"
	mysql_log "Note" "Check Liveness/Readiness Success"
	return_ok
fi

if [ $READINESS -eq 1 ]; then
    status=$($MYSQL_BIN $MYSQL_OPTS -e "$CHECK_QUERY;" 2>/dev/null | awk '{print $2;}')
	# A node is ready when it wsrep_ready=on
	if [ "$status" = "ON" ]; then
		#readonly=$($MYSQL_BIN $MYSQL_OPTS -e "$CHECK_QUERY2;" 2>/dev/null | awk '{print $2;}')

		#if [ "$readonly" = "YES" -o "$readonly" = "ON" ]; then
		#	mysql_log "Note" "Check Readiness Failed"
		#	return_fail
		#fi

		select=$($MYSQL_BIN $MYSQL_OPTS -e "$CHECK_SQL;" 2>/dev/null | awk '{print $1;}')
		if [[ !("$select" =~ "12345") ]]; then
		    mysql_log "Note" "Check Readiness Failed"
			return_fail
		fi

		mysql_log "Note" "Check Readiness Success"
		return_ok
	fi
	mysql_log "Note" "Check Readiness Failed"
fi

if [ $LIVENESS -eq 1 ]; then
	# A node is alive if it's not in Initialized state or Inconsistent
	comment_status=$($MYSQL_BIN $MYSQL_OPTS -e "$CHECK_QUERY3;" 2>/dev/null | awk '{print $2;}')
	if [ "$comment_status" != "Initialized" ] && [ "$comment_status" != "Inconsistent" ]; then
		mysql_log "Note" "Check Liveness Success"
		return_ok
	fi
	mysql_log "Warn" "Check Liveness Failed"
fi

mysql_log "Warn" "ALL Failed"
return_fail

