#!/bin/bash
# Report Galera status to etcd periodically.
# report_status.sh [mysql user] [mysql password] [cluster name] [interval] [comma separated etcd hosts]
# Example:
# report_status.sh root myS3cret galera_cluster 15 192.168.55.111:2379,192.168.55.112:2379,192.168.55.113:2379

MYSQL_BIN='/usr/bin/mariadb'
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3306"
MYSQL_USERNAME=${MYSQL_ROOT_USER}
MYSQL_PASSWORD=${MYSQL_ROOT_PASSWORD}
MYSQL_OPTS="-N -q -A --connect-timeout=10 --host=$MYSQL_HOST --port=$MYSQL_PORT --user=$MYSQL_USERNAME --password=$MYSQL_PASSWORD"

CLUSTER_NAME=$1
TTL=$2
ETCD_HOSTS=$3

# logging functions
mysql_log() {
	local type="$1"; shift
	local key="$1"; shift
	printf '%s [%s] [ReportStatus] [%s]: %s\n' "$(date --rfc-3339=seconds)" "$type" "$key" "$*" >> /var/lib/mysql/report_status.log
}

function report_status()
{
	var=$1
	key=$2

	if [ ! -z $var ]; then
		mysql_cmd="mariadb -A -Bse \"show status like '$var'\""
		mysql_log "Note" "mysql_cmd" $mysql_cmd

		output=$($MYSQL_BIN $MYSQL_OPTS -e "show status like '$var'" 2>&1)

		if [ $? -ne 0 ]; then
			mysql_log "Warn" "mysql" "$output"
			return 1
		fi

		if [ -z $key ]; then
			key=$(echo $output | awk {'print $1'})
		fi

		value=$(echo $output | awk {'print $2'})
		#ipaddr=$(hostname -i | awk {'print $1'})
		HOSTNAME=$(hostname)

		mysql_log "Note" "$key" "$value"
		if [ ! -z $value ]; then
			URL="http://$ETCD_HOSTS/v2/keys/galera/$CLUSTER_NAME/$HOSTNAME/$key"
			timeout 12 curl --max-time 10 -s $URL -X PUT -d "value=$value&ttl=$TTL" > /dev/null
		fi
	fi
}

mysql_log "Note" "CLUSTER_NAME" $CLUSTER_NAME
mysql_log "Note" "TTL" $TTL
mysql_log "Note" "ETCD_HOSTS" $ETCD_HOSTS

while true; do
	sleep $(($TTL - 2))
	mysql_log "Note" "report" "begin"

	report_status wsrep_local_state_comment
	report_status wsrep_last_committed seqno
	# report every ttl - 2 to ensure value does not expire
	mysql_log "Note" "report" "end"
done

