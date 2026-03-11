#!/bin/bash
set -eo pipefail
shopt -s nullglob

if [ "${1:0:1}" = '-' ]; then
	CMDARG="$@"
fi

if [ "${TRACE}" = "1" ]; then
	set -x
fi

# if command starts with an option, prepend mariadbd
if [ "${1:0:1}" = '-' ]; then
	set -- mariadbd "$@"
fi

[ -z "${TTL}" ] && TTL=2
WAIT=$((${TTL} * 10))
[ -z "${REPORT_TTL}" ] && REPORT_TTL=5

if [ -z "${MYSQL_ROOT_USER}" ]; then
	MYSQL_ROOT_USER=root
fi

if [ -z "${POD_NAME}" ]; then
	echo >&2 'Error: You need to specify POD_NAME'
	exit 1
fi

if [ -z "${CLUSTER_NAME}" ]; then
	echo >&2 'Error: You need to specify CLUSTER_NAME'
	exit 1
fi

if [ -z "${SERVICE_NAME}" ]; then
	echo >&2 'Error: You need to specify SERVICE_NAME'
	exit 1
fi

if [ -z "${STATEFULSET_NAME}" ]; then
	echo >&2 'Error: You need to specify STATEFULSET_NAME'
	exit 1
fi

if [ -z "${CLUSTER_SIZE}" ]; then
	echo >&2 'Error: You need to specify CLUSTER_SIZE'
	exit 1
fi

sleep ${SLEEP}

#IPADDR=$(hostname -I | awk {'print $1'})
HOSTNAME=${POD_NAME}
HEADLESS=${HOSTNAME}.${SERVICE_NAME}

#SET WSREP-CLUSTER-ADDRESS=gcomm://galera-ss-0.galera-ss,galera-ss-1.galera-ss,galera-ss-2.galera-ss,
#SET WSREP-SST-DONOR=galera-ss-2.galera-ss,galera-ss-1.galera-ss,galera-ss-0.galera-ss,
PEERS=""
WSREP_SST_DONOR=""
for i in $(seq 0 $((${CLUSTER_SIZE} - 1))); do
	PEERS="${PEERS}${STATEFULSET_NAME}-${i}.${SERVICE_NAME},"
	WSREP_SST_DONOR="${WSREP_SST_DONOR}${STATEFULSET_NAME}-$((CLUSTER_SIZE - 1 - i)).${SERVICE_NAME},"
done

# logging functions
mysql_log() {
	local type="$1"; shift
	printf '%s [%s] [Entrypoint]: %s\n' "$(date --rfc-3339=seconds)" "$type" "$*"
}
mysql_note() {
	mysql_log Note "$@"
}
mysql_warn() {
	mysql_log Warn "$@" >&2
}
mysql_error() {
	mysql_log ERROR "$@" >&2
	exit 1
}

check_dns() {
	mysql_note "step into check dns"
	while true
	do
		dns_err=`curl ${HEADLESS} 2>&1 | grep 'Could not resolve host'` || echo ''
		[ -z "${dns_err}" ] && break
		mysql_warn ${dns_err}
		sleep 1
	done
}

# usage: file_env VAR [DEFAULT]
#    ie: file_env 'XYZ_DB_PASSWORD' 'example'
# (will allow for "${XYZ_DB_PASSWORD_FILE}" to fill in the value of
#  "${XYZ_DB_PASSWORD}" from a file, especially for Docker's secrets feature)
file_env() {
	mysql_note "step into file_env"
	local var="$1"
	local fileVar="${var}_FILE"
	local def="${2:-}"
	if [ "${!var:-}" ] && [ "${!fileVar:-}" ]; then
		mysql_error "Both $var and $fileVar are set (but are exclusive)"
	fi
	local val="$def"
	if [ "${!var:-}" ]; then
		val="${!var}"
	elif [ "${!fileVar:-}" ]; then
		val="$(< "${!fileVar}")"
	fi
	export "$var"="$val"
	unset "$fileVar"
}

# set MARIADB_xyz from MYSQL_xyz when MARIADB_xyz is unset
# and make them the same value (so user scripts can use either)
_mariadb_file_env() {
	local var="$1"; shift
	local maria="MARIADB_${var#MYSQL_}"
	file_env "$var" "$@"
	file_env "$maria" "${!var}"
	if [ "${!maria:-}" ]; then
		export "$var"="${!maria}"
	fi
}

# check to see if this file is being run or sourced from another script
_is_sourced() {
	mysql_note "step into _is_sourced"
	# https://unix.stackexchange.com/a/215279
	[ "${#FUNCNAME[@]}" -ge 2 ] \
		&& [ "${FUNCNAME[0]}" = '_is_sourced' ] \
		&& [ "${FUNCNAME[1]}" = 'source' ]
}

# usage: docker_process_init_files [file [file [...]]]
#    ie: docker_process_init_files /always-initdb.d/*
# process initializer files, based on file extensions
docker_process_init_files() {
	# mysql here for backwards compatibility "${mysql[@]}"
	# ShellCheck: mysql appears unused. Verify use (or export if used externally)
	# shellcheck disable=SC2034
	mysql=( docker_process_sql )

	echo
	local f
	for f; do
		case "$f" in
			*.sh)
				# https://github.com/docker-library/postgres/issues/450#issuecomment-393167936
				# https://github.com/docker-library/postgres/pull/452
				if [ -x "$f" ]; then
					mysql_note "$0: running $f"
					"$f"
				else
					mysql_note "$0: sourcing $f"
					# ShellCheck can't follow non-constant source. Use a directive to specify location.
					# shellcheck disable=SC1090
					. "$f"
				fi
				;;
			*.sql)     mysql_note "$0: running $f"; docker_process_sql < "$f"; echo ;;
			*.sql.gz)  mysql_note "$0: running $f"; gunzip -c "$f" | docker_process_sql; echo ;;
			*.sql.xz)  mysql_note "$0: running $f"; xzcat "$f" | docker_process_sql; echo ;;
			*.sql.zst) mysql_note "$0: running $f"; zstd -dc "$f" | docker_process_sql; echo ;;
			*)         mysql_warn "$0: ignoring $f" ;;
		esac
		echo
	done
}

# arguments necessary to run "mariadbd --verbose --help" successfully (used for testing configuration validity and for extracting default/configured values)
_verboseHelpArgs=(
	--verbose --help
	--log-bin-index="$(mktemp -u)" # https://github.com/docker-library/mysql/issues/136
	--encrypt-tmp-files=0 # https://github.com/docker-library/mariadb/issues/339
	--wsrep-on=0
)

mysql_check_config() {
	mysql_note "step into mysql_check_config"
	local toRun=( "$@" "${_verboseHelpArgs[@]}" ) errors
	if ! errors="$("${toRun[@]}" 2>&1 >/dev/null)"; then
		mysql_error $'mariadbd failed while attempting to check config\n\tcommand was: '"${toRun[*]}"$'\n\t'"$errors"
	fi
}

# Fetch value from server config
# We use mariadbd --verbose --help instead of my_print_defaults because the
# latter only show values present in config files, and not server defaults
mysql_get_config() {
	local conf="$1"; shift
	"$@" "${_verboseHelpArgs[@]}" 2>/dev/null \
		| awk -v conf="$conf" '$1 == conf && /^[^ \t]/ { sub(/^[^ \t]+[ \t]+/, ""); print; exit }'
	# match "datadir      /some/path with/spaces in/it here" but not "--xyz=abc\n     datadir (xyz)"
}

# Do a temporary startup of the MariaDB server, for init purposes
docker_temp_server_start() {
	mysql_note "step into docker_temp_server_start"
	"$@" --skip-networking --socket="${SOCKET}" --wsrep-on=0 &
	mysql_note "Waiting for server startup"
	declare -g MARIADB_PID
	MARIADB_PID=$!
	mysql_note "Waiting for server startup"
	# only use the root password if the database has already been initialized
	# so that it won't try to fill in a password file when it hasn't been set yet
	extraArgs=()
	if [ -z "$DATABASE_ALREADY_EXISTS" ]; then
		extraArgs+=( '--dont-use-mysql-root-password' )
	fi
	local i
	for i in {30..0}; do
		if docker_process_sql "${extraArgs[@]}" --database=mysql <<<'SELECT 1' &> /dev/null; then
			break
		fi
		sleep 1
	done
	if [ "$i" = 0 ]; then
		mysql_error "Unable to start server."
	fi
}

# Stop the server. When using a local socket file mysqladmin will block until
# the shutdown is complete.
docker_temp_server_stop() {
	mysql_note "step into docker_temp_server_stop"
	kill "$MARIADB_PID"
	wait "$MARIADB_PID"
}

# Verify that the minimally required password settings are set for new databases.
docker_verify_minimum_env() {
	mysql_note "step into docker_verify_minimum_env"
	if [ -z "$MARIADB_ROOT_PASSWORD" ] && [ -z "$MARIADB_ROOT_PASSWORD_HASH" ] && [ -z "$MARIADB_ALLOW_EMPTY_ROOT_PASSWORD" ] && [ -z "$MARIADB_RANDOM_ROOT_PASSWORD" ]; then
		mysql_error $'Database is uninitialized and password option is not specified\n\tYou need to specify one of MARIADB_ROOT_PASSWORD, MARIADB_ROOT_PASSWORD_HASH, MARIADB_ALLOW_EMPTY_ROOT_PASSWORD and MARIADB_RANDOM_ROOT_PASSWORD'
	fi
	# More preemptive exclusions of combinations should have been made before *PASSWORD_HASH was added, but for now we don't enforce due to compatibility.
	if [ -n "$MARIADB_ROOT_PASSWORD" ] || [ -n "$MARIADB_ALLOW_EMPTY_ROOT_PASSWORD" ] || [ -n "$MARIADB_RANDOM_ROOT_PASSWORD" ] && [ -n "$MARIADB_ROOT_PASSWORD_HASH" ]; then
		mysql_error "Cannot specify MARIADB_ROOT_PASSWORD_HASH and another MARIADB_ROOT_PASSWORD* option."
	fi
	if [ -n "$MARIADB_PASSWORD" ] && [ -n "$MARIADB_PASSWORD_HASH" ]; then
		mysql_error "Cannot specify MARIADB_PASSWORD_HASH and MARIADB_PASSWORD option."
	fi
	if [ -n "$MARIADB_REPLICATION_USER" ]; then
		if [ -z "$MARIADB_MASTER_HOST" ]; then
			# its a master, we're creating a user
			if [ -z "$MARIADB_REPLICATION_PASSWORD" ] && [ -z "$MARIADB_REPLICATION_PASSWORD_HASH" ]; then
				mysql_error "MARIADB_REPLICATION_PASSWORD or MARIADB_REPLICATION_PASSWORD_HASH not found to create replication user for master"
			fi
		else
			# its a replica
			if [ -z "$MARIADB_REPLICATION_PASSWORD" ] ; then
				mysql_error "MARIADB_REPLICATION_PASSWORD is mandatory to specify the replication on the replica image."
			fi
			if [ -n "$MARIADB_REPLICATION_PASSWORD_HASH" ] ; then
				mysql_warn "MARIADB_REPLICATION_PASSWORD_HASH cannot be specified on a replica"
			fi
		fi
	fi
	if [ -n "$MARIADB_MASTER_HOST" ] && { [ -z "$MARIADB_REPLICATION_USER" ] || [ -z "$MARIADB_REPLICATION_PASSWORD" ] ; }; then
		mysql_error "For a replica, MARIADB_REPLICATION_USER and MARIADB_REPLICATION is mandatory."
	fi
}

# creates folders for the database
# also ensures permission for user mysql of run as root
docker_create_db_directories() {
	mysql_note "step into docker_create_db_directories"
	local user; user="$(id -u)"
	mysql_note "current user is $user"

	# TODO other directories that are used by default? like /var/lib/mysql-files
	# see https://github.com/docker-library/mysql/issues/562
	mkdir -p "${DATADIR}"

	if [ "$user" = "0" ]; then
		# this will cause less disk access than `chown -R`
		find "${DATADIR}" \! -user mysql -exec chown mysql '{}' +
		# See https://github.com/MariaDB/mariadb-docker/issues/363
		find "${SOCKET%/*}" -maxdepth 0 \! -user mysql -exec chown mysql '{}' \;
	fi
	mysql_note "datadir is ${DATADIR}"
}

_mariadb_version() {
        local mariaVersion="${MARIADB_VERSION##*:}"
        mariaVersion="${mariaVersion%%[-+~]*}"
	echo -n "${mariaVersion}-MariaDB"
}

# initializes the database directory
docker_init_database_dir() {
	mysql_note "step into docker_init_database_dir"
	mysql_note "Initializing database files"
	installArgs=( --datadir="${DATADIR}" --rpm --auth-root-authentication-method=normal --wsrep-on=0)
	mariadb-install-db "${installArgs[@]}" "${@:2}" \
		--skip-test-db \
		--old-mode='UTF8_IS_UTF8MB3' \
		--default-time-zone=SYSTEM --enforce-storage-engine= \
		--skip-log-bin \
		--expire-logs-days=0 \
		--loose-innodb_buffer_pool_load_at_startup=0 \
		--loose-innodb_buffer_pool_dump_at_shutdown=0
	mysql_note "Database files initialized"
}

# Loads various settings that are used elsewhere in the script
# This should be called after mysql_check_config, but before any other functions
docker_setup_env() {
	mysql_note "step into docker_setup_env"
	# Get config
	declare -g DATADIR SOCKET TMP
	DATADIR="$(mysql_get_config 'datadir' "$@")"
	SOCKET="$(mysql_get_config 'socket' "$@")"
	PORT="$(mysql_get_config 'port' "$@")"
	TMP=${DATADIR}/`date +%Y-%m-%d_%H:%M:%S`.recover


	# Initialize values that might be stored in a file
	_mariadb_file_env 'MYSQL_ROOT_HOST' '127.0.0.1'
	_mariadb_file_env 'MYSQL_DATABASE'
	_mariadb_file_env 'MYSQL_USER'
	_mariadb_file_env 'MYSQL_PASSWORD'
	_mariadb_file_env 'MYSQL_ROOT_USER'
	_mariadb_file_env 'MYSQL_ROOT_PASSWORD'
	# No MYSQL_ compatibility needed for new variables
	file_env 'MARIADB_PASSWORD_HASH'
	file_env 'MARIADB_ROOT_PASSWORD_HASH'
	# env variables related to replication
	file_env 'MARIADB_REPLICATION_USER'
	file_env 'MARIADB_REPLICATION_PASSWORD'
	file_env 'MARIADB_REPLICATION_PASSWORD_HASH'
	# env variables related to master
	file_env 'MARIADB_MASTER_HOST'
	file_env 'MARIADB_MASTER_PORT' 3306

	# set MARIADB_ from MYSQL_ when it is unset and then make them the same value
	: "${MARIADB_ALLOW_EMPTY_ROOT_PASSWORD:=${MYSQL_ALLOW_EMPTY_PASSWORD:-}}"
	export MYSQL_ALLOW_EMPTY_PASSWORD="$MARIADB_ALLOW_EMPTY_ROOT_PASSWORD" MARIADB_ALLOW_EMPTY_ROOT_PASSWORD
	: "${MARIADB_RANDOM_ROOT_PASSWORD:=${MYSQL_RANDOM_ROOT_PASSWORD:-}}"
	export MYSQL_RANDOM_ROOT_PASSWORD="$MARIADB_RANDOM_ROOT_PASSWORD" MARIADB_RANDOM_ROOT_PASSWORD
	: "${MARIADB_INITDB_SKIP_TZINFO:=${MYSQL_INITDB_SKIP_TZINFO:-}}"
	export MYSQL_INITDB_SKIP_TZINFO="$MARIADB_INITDB_SKIP_TZINFO" MARIADB_INITDB_SKIP_TZINFO

	declare -g DATABASE_ALREADY_EXISTS
	if [ -d "$DATADIR/mysql" ]; then
		DATABASE_ALREADY_EXISTS='true'
	fi
}

# Execute the client, use via docker_process_sql to handle root password
docker_exec_client() {
	# args sent in can override this db, since they will be later in the command
	if [ -n "$MYSQL_DATABASE" ]; then
		set -- --database="$MYSQL_DATABASE" "$@"
	fi
	mariadb --protocol=socket -uroot -hlocalhost --socket="${SOCKET}" "$@"
}

# Execute sql script, passed via stdin
# usage: docker_process_sql [--dont-use-mysql-root-password] [mysql-cli-args]
#    ie: docker_process_sql --database=mydb <<<'INSERT ...'
#    ie: docker_process_sql --dont-use-mysql-root-password --database=mydb <my-file.sql
docker_process_sql() {
	passfileArgs=()
	if [ '--dont-use-mysql-root-password' = "$1" ]; then
		shift
		MYSQL_PWD= docker_exec_client "$@"
	else
		MYSQL_PWD=$MARIADB_ROOT_PASSWORD docker_exec_client "$@"
	fi
}

# SQL escape the string $1 to be placed in a string literal.
# escape, \ followed by '
docker_sql_escape_string_literal() {
	local newline=$'\n'
	local escaped=${1//\\/\\\\}
	escaped="${escaped//$newline/\\n}"
	echo "${escaped//\'/\\\'}"
}

# Creates replication user
create_replica_user() {
	if [ -n  "$MARIADB_REPLICATION_PASSWORD_HASH" ]; then
		echo "CREATE USER '$MARIADB_REPLICATION_USER'@'%' IDENTIFIED BY PASSWORD '$MARIADB_REPLICATION_PASSWORD_HASH';"
	else
		# SQL escape the user password, \ followed by '
		local userPasswordEscaped
		userPasswordEscaped=$( docker_sql_escape_string_literal "${MARIADB_REPLICATION_PASSWORD}" )
		echo "CREATE USER '$MARIADB_REPLICATION_USER'@'%' IDENTIFIED BY '$userPasswordEscaped';"
	fi
	echo "GRANT REPLICATION REPLICA ON *.* TO '$MARIADB_REPLICATION_USER'@'%';"
}


# Initializes database with timezone info and root password, plus optional extra db/user
docker_setup_db() {
	mysql_note "step into docker_setup_db"
	# Load timezone info into database
	if [ -n "${TZ}" ]; then
		TZNAME=$(echo ${TZ} | cut -d '/' -f 2)
		mysql_note "start set timezone ${TZ}:${TZNAME}"
		{
			# Aria in 10.4+ is slow due to "transactional" (crash safety)
			# https://jira.mariadb.org/browse/MDEV-23326
			# https://github.com/docker-library/mariadb/issues/262
			local tztables=( time_zone time_zone_leap_second time_zone_name time_zone_transition time_zone_transition_type )
			for table in "${tztables[@]}"; do
				echo "/*!100400 ALTER TABLE $table TRANSACTIONAL=0 */;"
			done

			# sed is for https://bugs.mysql.com/bug.php?id=20545
			mariadb-tzinfo-to-sql /usr/share/zoneinfo/${TZ} ${TZNAME} \
				| sed 's/Local time zone must be set--see zic manual page/FCTY/'

			for table in "${tztables[@]}"; do
				echo "/*!100400 ALTER TABLE $table TRANSACTIONAL=1 */;"
			done
		} | docker_process_sql --dont-use-mysql-root-password --database=mysql
		# tell docker_process_sql to not use MYSQL_ROOT_PASSWORD since it is not set yet

	elif [ -z "${MARIADB_INITDB_SKIP_TZINFO}" ]; then
		# --skip-write-binlog usefully disables binary logging
		# but also outputs LOCK TABLES to improve the IO of
		# Aria (MDEV-23326) for 10.4+.
		mariadb-tzinfo-to-sql --skip-write-binlog /usr/share/zoneinfo \
			| docker_process_sql --dont-use-mysql-root-password --database=mysql
		# tell docker_process_sql to not use MYSQL_ROOT_PASSWORD since it is not set yet
	fi

	mysql_note "start Generate root password"
	# Generate random root password
	if [ -n "${MARIADB_RANDOM_ROOT_PASSWORD}" ]; then
		export MARIADB_ROOT_PASSWORD="$(pwgen --numerals --capitalize --symbols --remove-chars="'\\" -1 32)"
		export MYSQL_ROOT_PASSWORD=${MARIADB_ROOT_PASSWORD}
		mysql_note "GENERATED ROOT PASSWORD: ${MARIADB_ROOT_PASSWORD}"
	fi

	# Sets root password and creates root users for non-localhost hosts
	local rootCreate=
	local rootPasswordEscaped=
	if [ -n "$MARIADB_ROOT_PASSWORD" ]; then
		# Sets root password and creates root users for non-localhost hosts
		rootPasswordEscaped=$( docker_sql_escape_string_literal "${MARIADB_ROOT_PASSWORD}" )
	fi

	# default root to listen for connections from anywhere
	if [ -n "$MARIADB_ROOT_HOST" ] && [ "$MARIADB_ROOT_HOST" != 'localhost' ]; then
		# ref "read -d ''", no, we don't care if read finds a terminating character in this heredoc
		# https://unix.stackexchange.com/questions/265149/why-is-set-o-errexit-breaking-this-read-heredoc-expression/265151#265151
		if [ -n "$MARIADB_ROOT_PASSWORD_HASH" ]; then
			read -r -d '' rootCreate <<-EOSQL || true
				CREATE USER 'root'@'${MARIADB_ROOT_HOST}' IDENTIFIED BY PASSWORD '${MARIADB_ROOT_PASSWORD_HASH}' ;
				GRANT ALL ON *.* TO 'root'@'${MARIADB_ROOT_HOST}' WITH GRANT OPTION ;
			EOSQL
		else
			read -r -d '' rootCreate <<-EOSQL || true
				CREATE USER 'root'@'${MARIADB_ROOT_HOST}' IDENTIFIED BY '${rootPasswordEscaped}' ;
				GRANT ALL ON *.* TO 'root'@'${MARIADB_ROOT_HOST}' WITH GRANT OPTION ;
			EOSQL
		fi
	fi

	local mysqlAtLocalhost=
	local mysqlAtLocalhostGrants=
	# Install mysql@localhost user
	if [ -n "$MARIADB_MYSQL_LOCALHOST_USER" ]; then
		read -r -d '' mysqlAtLocalhost <<-EOSQL || true
		CREATE USER mysql@localhost IDENTIFIED VIA unix_socket;
		EOSQL
		if [ -n "$MARIADB_MYSQL_LOCALHOST_GRANTS" ]; then
			if [ "$MARIADB_MYSQL_LOCALHOST_GRANTS" != USAGE ]; then
				mysql_warn "Excessive privileges ON *.* TO mysql@localhost facilitates risks to the confidentiality, integrity and availability of data stored"
			fi
			mysqlAtLocalhostGrants="GRANT ${MARIADB_MYSQL_LOCALHOST_GRANTS} ON *.* TO mysql@localhost;";
		fi
	fi

	local healthCheckUser
	local healthCheckGrant=USAGE
	local healthCheckConnectPass
	local healthCheckConnectPassEscaped
	healthCheckConnectPass="$(pwgen --numerals --capitalize --symbols --remove-chars="=#'\\" -1 32)"
	healthCheckConnectPassEscaped=$( docker_sql_escape_string_literal "${healthCheckConnectPass}" )
	if [ -n "$MARIADB_HEALTHCHECK_GRANTS" ]; then
		healthCheckGrant="$MARIADB_HEALTHCHECK_GRANTS"
	fi
	read -r -d '' healthCheckUser <<-EOSQL || true
	CREATE USER healthcheck@'127.0.0.1' IDENTIFIED BY '$healthCheckConnectPassEscaped';
	CREATE USER healthcheck@'::1' IDENTIFIED BY '$healthCheckConnectPassEscaped';
	CREATE USER healthcheck@localhost IDENTIFIED BY '$healthCheckConnectPassEscaped';
	GRANT $healthCheckGrant ON *.* TO healthcheck@'127.0.0.1';
	GRANT $healthCheckGrant ON *.* TO healthcheck@'::1';
	GRANT $healthCheckGrant ON *.* TO healthcheck@localhost;
	EOSQL
	local maskPreserve
	maskPreserve=$(umask -p)
	umask 0077
	# ignore Docker Official Images healthcheck.sh script
	# echo -e "[mariadb-client]\\nport=$PORT\\nsocket=$SOCKET\\nuser=healthcheck\\npassword=$healthCheckConnectPass\\nprotocol=tcp\\n" > "$DATADIR"/.my-healthcheck.cnf
	$maskPreserve

	local rootLocalhostPass=
	if [ -z "$MARIADB_ROOT_PASSWORD_HASH" ]; then
		# handle MARIADB_ROOT_PASSWORD_HASH for root@localhost after /docker-entrypoint-initdb.d
		rootLocalhostPass="SET PASSWORD FOR 'root'@'localhost'= PASSWORD('${rootPasswordEscaped}');"
	fi

	local createDatabase=
	# Creates a custom database and user if specified
	if [ -n "$MARIADB_DATABASE" ]; then
		mysql_note "Creating database ${MARIADB_DATABASE}"
		createDatabase="CREATE DATABASE IF NOT EXISTS \`$MARIADB_DATABASE\`;"
	fi

	local createUser=
	local userGrants=
	if  [ -n "$MARIADB_PASSWORD" ] || [ -n "$MARIADB_PASSWORD_HASH" ] && [ -n "$MARIADB_USER" ]; then
		mysql_note "Creating user ${MARIADB_USER}"
		if [ -n "$MARIADB_PASSWORD_HASH" ]; then
			createUser="CREATE USER '$MARIADB_USER'@'%' IDENTIFIED BY PASSWORD '$MARIADB_PASSWORD_HASH';"
		else
			# SQL escape the user password, \ followed by '
			local userPasswordEscaped
			userPasswordEscaped=$( docker_sql_escape_string_literal "${MARIADB_PASSWORD}" )
			createUser="CREATE USER '$MARIADB_USER'@'%' IDENTIFIED BY '$userPasswordEscaped';"
		fi

		if [ -n "$MARIADB_DATABASE" ]; then
			mysql_note "Giving user ${MARIADB_USER} access to schema ${MARIADB_DATABASE}"
			userGrants="GRANT ALL ON \`${MARIADB_DATABASE//_/\\_}\`.* TO '$MARIADB_USER'@'%';"
		fi
	fi

	# To create replica user
	local createReplicaUser=
	local changeMasterTo=
	local startReplica=
	if  [ -n "$MARIADB_REPLICATION_USER" ] ; then
		if [ -z "$MARIADB_MASTER_HOST" ]; then
			# on master
			mysql_note "Creating user ${MARIADB_REPLICATION_USER}"
			createReplicaUser=$(create_replica_user)
		else
			# on replica
			local rplPasswordEscaped
			rplPasswordEscaped=$( docker_sql_escape_string_literal "${MARIADB_REPLICATION_PASSWORD}" )
			# SC cannot follow how MARIADB_MASTER_PORT is assigned a default value.
			# shellcheck disable=SC2153
			changeMasterTo="CHANGE MASTER TO MASTER_HOST='$MARIADB_MASTER_HOST', MASTER_USER='$MARIADB_REPLICATION_USER', MASTER_PASSWORD='$rplPasswordEscaped', MASTER_PORT=$MARIADB_MASTER_PORT, MASTER_CONNECT_RETRY=10;"
			startReplica="START REPLICA;"
		fi
	fi

	# create root@'%' to manage db
	# create safety user to prevent password loss
	local rdsUser
	read -r -d '' rdsUser <<-EOSQL || true
	CREATE USER '${MYSQL_ROOT_USER}'@'%' IDENTIFIED BY '${MYSQL_ROOT_PASSWORD}';
	GRANT ALL ON *.* TO '${MYSQL_ROOT_USER}'@'%' WITH GRANT OPTION ;
	GRANT ALL ON *.* TO 'safety'@'127.0.0.1' IDENTIFIED BY 'fakepassword' WITH GRANT OPTION ;
	GRANT ALL ON *.* TO 'safety'@'localhost' IDENTIFIED BY 'fakepassword' WITH GRANT OPTION ;
	CREATE USER 'monitor'@'%' IDENTIFIED BY 'fakepassword';
	EOSQL

	mysql_note "Securing system users (equivalent to running mysql_secure_installation)"
	# tell docker_process_sql to not use MARIADB_ROOT_PASSWORD since it is just now being set
	# --binary-mode to save us from the semi-mad users go out of their way to confuse the encoding.
	docker_process_sql --dont-use-mysql-root-password --database=mysql --binary-mode <<-EOSQL
		-- Securing system users shouldn't be replicated
		SET @orig_sql_log_bin= @@SESSION.SQL_LOG_BIN;
		SET @@SESSION.SQL_LOG_BIN=0;
                -- we need the SQL_MODE NO_BACKSLASH_ESCAPES mode to be clear for the password to be set
		SET @@SESSION.SQL_MODE=REPLACE(@@SESSION.SQL_MODE, 'NO_BACKSLASH_ESCAPES', '');

		DROP USER IF EXISTS root@'127.0.0.1', root@'::1';
		EXECUTE IMMEDIATE CONCAT('DROP USER IF EXISTS root@\'', @@hostname,'\'');

		${rootLocalhostPass}
		${rootCreate}
		${mysqlAtLocalhost}
		${mysqlAtLocalhostGrants}
		-- end of securing system users, rest of init now...
		SET @@SESSION.SQL_LOG_BIN=@orig_sql_log_bin;
		-- create users/databases
		${createDatabase}
		${createUser}
		${createReplicaUser}
		${userGrants}
		${rdsUser}

		${changeMasterTo}
		${startReplica}
	EOSQL

	mysql_note "step out of docker_setup_db"
}

# backup the mysql database
docker_mariadb_backup_system()
{
	if [ -n "$MARIADB_DISABLE_UPGRADE_BACKUP" ] \
		&& [ "$MARIADB_DISABLE_UPGRADE_BACKUP" = 1 ]; then
		mysql_note "MariaDB upgrade backup disabled due to \$MARIADB_DISABLE_UPGRADE_BACKUP=1 setting"
		return
	fi
	local backup_db="system_mysql_backup_unknown_version.sql.zst"
	local oldfullversion="unknown_version"
	if [ -r "$DATADIR"/mysql_upgrade_info ]; then
		read -r -d '' oldfullversion < "$DATADIR"/mysql_upgrade_info || true
		if [ -n "$oldfullversion" ]; then
			backup_db="system_mysql_backup_${oldfullversion}.sql.zst"
		fi
	fi

	mysql_note "Backing up system database to $backup_db"
	if ! mariadb-dump --skip-lock-tables --replace --databases mysql --socket="${SOCKET}" | zstd > "${DATADIR}/${backup_db}"; then
		mysql_error "Unable backup system database for upgrade from $oldfullversion."
	fi
	mysql_note "Backing up complete"
}

# perform mariadb-upgrade
docker_mariadb_upgrade() {
	mysql_note "step into docker_mariadb_upgrade"
	#ignore MARIADB_AUTO_UPGRADE
	#if [ -z "$MARIADB_AUTO_UPGRADE" ] \
	#	|| [ "$MARIADB_AUTO_UPGRADE" = 0 ]; then
	#	mysql_note "MariaDB upgrade (mariadb-upgrade) required, but skipped due to \$MARIADB_AUTO_UPGRADE setting"
	#	return
	#fi
	if [ -n "$SKIP_MARIADB_AUTO_UPGRADE" ]; then
	    mysql_note "MariaDB upgrade (mariadb-upgrade) required, but skipped due to \$SKIP_MARIADB_AUTO_UPGRADE setting"
		return
	fi
	mysql_note "Starting temporary server"
	docker_temp_server_start "$@" --skip-grant-tables \
		--loose-innodb_buffer_pool_dump_at_shutdown=0 \
		--skip-slave-start
	mysql_note "Temporary server started."

	# docker_mariadb_backup_system

	mysql_note "Starting mariadb-upgrade"
	mariadb-upgrade --upgrade-system-tables
	mysql_note "Finished mariadb-upgrade"

	mysql_note "Stopping temporary server"
	docker_temp_server_stop
	mysql_note "Temporary server stopped"
}


_check_if_upgrade_is_needed() {
	mysql_note "step into _check_if_upgrade_is_needed"
	if [ ! -f "$DATADIR"/mysql_upgrade_info ]; then
		mysql_note "MariaDB upgrade information missing, assuming required"
		return 0
	fi
	local mariadbVersion
	mariadbVersion="$(_mariadb_version)"
	IFS='.-' read -ra newversion <<<"$mariadbVersion"
	IFS='.-' read -ra oldversion < "$DATADIR"/mysql_upgrade_info || true

	if [[ ${#newversion[@]} -lt 2 ]] || [[ ${#oldversion[@]} -lt 2 ]] \
		|| [[ ${oldversion[0]} -lt ${newversion[0]} ]] \
		|| [[ ${oldversion[0]} -eq ${newversion[0]} && ${oldversion[1]} -lt ${newversion[1]} ]]; then
		return 0
	fi
	mysql_note "MariaDB upgrade not required"
	return 1
}

# check arguments for an option that would cause mariadbd to stop
# return true if there is one
_mysql_want_help() {
	mysql_note "step into _mysql_want_help"
	local arg
	for arg; do
		case "$arg" in
			-'?'|--help|--print-defaults|-V|--version)
				return 0
				;;
		esac
	done
	return 1
}

function join { local IFS="$1"; shift; echo "$*"; }

add_wsrep_conf() {
	mysql_note "step into add_wsrep_conf"

	# set IP address based on the primary interface
	sed -i "s|WSREP_NODE_ADDRESS|${HEADLESS}|g" /etc/mysql/conf.d/my.cnf
	sed -i "s|WSREP_SST_DONOR|${WSREP_SST_DONOR}|g" /etc/mysql/conf.d/my.cnf

	# support ipv6
	if [[ "$KUBERNETES_SERVICE_HOST" =~ ":" ]]; then
	    echo -e '[SST]\nsockopt=",pf=ip6"' >> /etc/mysql/conf.d/my.cnf
	fi
}

check_safe_bootstrap() {
	mysql_note "step into check_safe_bootstrap"
	GRASTATE=${DATADIR}/grastate.dat
	safe_to_bootstrap="0"
	[ -f ${GRASTATE} ] && safe_to_bootstrap=`sed -n 's/safe_to_bootstrap: //p' ${GRASTATE}`
	if [ "$safe_to_bootstrap" = "1" ]
	then
		nohup /report_status.sh ${CLUSTER_NAME} ${REPORT_TTL} ${DISCOVERY_SERVICE} &
		exec mariadbd --wsrep-cluster-name=${CLUSTER_NAME} --wsrep-cluster-address="gcomm://${PEERS}" --wsrep-new-cluster ${CMDARG}
	fi
}

check_force_bootstrap() {
	mysql_note "step into check_force_bootstrap"
	if [ -f "${DATADIR}/enforce_bootstrap" ]; then
		mysql_note "Enforce to bootstrap the cluster"
		rm -rf "${DATADIR}/enforce_bootstrap"
		GRASTATE=${DATADIR}/grastate.dat
		[ -f ${GRASTATE} ] && sed -i "s|safe_to_bootstrap.*|safe_to_bootstrap: 1|g" ${GRASTATE}
		nohup /report_status.sh ${CLUSTER_NAME} ${REPORT_TTL} ${DISCOVERY_SERVICE} &
		exec mariadbd --wsrep-cluster-name=${CLUSTER_NAME} --wsrep-cluster-address="gcomm://${PEERS}" --wsrep-new-cluster ${CMDARG}
	fi
}

#select max seqno as bootstraper, other nodes run as follower
get_cluster_join() {
	mysql_note "step into get_cluster_join"
	if [ -z "${DISCOVERY_SERVICE}" ]; then
		cluster_join=${CLUSTER_JOIN}
		return 0
	fi

	mysql_note "Try to determine local Galera sequence number."
	#mysqld_safe --wsrep-cluster-address=gcomm:// --wsrep-recover --skip-syslog
	#TMP=${DATADIR}/$(hostname).err
	#cat ${TMP}
	#seqno=$(cat ${TMP} | tr ' ' "\n" | grep -e '[a-z0-9]*-[a-z0-9]*:[-\0-9]' | tail -1 | cut -d ":" -f 2)
	timeout -s 9 60 mariadbd --user=mysql --wsrep-recover --wsrep-cluster-address=gcomm:// --log_error="${TMP}" || echo $?
	cat ${TMP}
	seqno=$(cat ${TMP} | tr ' ' "\n" | grep -e '[a-z0-9]*-[a-z0-9]*:[-\0-9]' | tail -1 | cut -d ":" -f 2 | cut -d "," -f 1)
	if [ -z ${seqno} ]; then
		mysql_error "Unable to determine Galera sequence number."
		exit 1
	fi
	mysql_note "Determine local Galera sequence number: (${seqno})"

	mysql_note "Waiting for ${REPORT_TTL} seconds to read non-expired keys.."
	sleep ${REPORT_TTL}

	URL="http://${DISCOVERY_SERVICE}/v2/keys/galera/${CLUSTER_NAME}"
	while true; do
		mysql_note "Try to found running nodes"
		mysql_note "Reporting seqno:${seqno} to ${DISCOVERY_SERVICE}."
		mysql_note "URL: ${URL}/${HOSTNAME}/seqno"
		curl -s --connect-timeout 1 --retry 30 --retry-delay 1 ${URL}/${HOSTNAME}/seqno -X PUT -d "value=${seqno}&ttl=${WAIT}"

		curl -s "${URL}?recursive=true&sorted=true" > /tmp/out
		etcd_rslt=$(cat /tmp/out)
		mysql_note "etcd status: ${etcd_rslt}"

		running_nodes=$(cat /tmp/out | jq -r '.node.nodes[].nodes[]? | select(.key | contains ("wsrep_local_state_comment")) | select(.value == "Synced") | .key' | awk -F'/' '{print $(NF-1)}' | tr "\n" ' '| sed -e 's/[[:space:]]*$//')
		mysql_note "Running nodes: [${running_nodes}]"

		if [ ! -z "${running_nodes}" ]; then
			cluster_join=$(join , ${running_nodes})
			break
		fi

		mysql_warn "Not found running nodes, try to bootstrap"
		curl -s "${URL}?recursive=true&sorted=true" > /tmp/out
		cluster_size=$(cat /tmp/out | jq -r '.node.nodes[].nodes[]? | select(.key | contains ("seqno")) | .value' | wc -l)
		cluster_member="$(cat /tmp/out | jq -r '.node.nodes[].nodes[]? | select(.key | contains ("seqno")) | .key')"
		if [ "${cluster_size}" -ne "${CLUSTER_SIZE}" ]; then
			mysql_warn "Not all nodes reported. Reported: "
			mysql_warn "${cluster_member}"
			sleep $((${RANDOM} % 3))
			continue
		fi

		mysql_note "All nodes have reported seqno"
		cluster_seqno=$(cat /tmp/out | jq -r '.node.nodes[].nodes[]? | select(.key | contains ("seqno")) | .value' | tr "\n" ' '| sed -e 's/[[:space:]]*$//')
		max_seqno=-1
		for i in ${cluster_seqno}; do
			if [ $i -gt ${max_seqno} ]; then
				max_seqno=$i
			fi
		done

		mysql_note "Found the greatest seqno: (${max_seqno})"
		min_ip=$(cat /tmp/out | jq -r ".node.nodes[].nodes[]? | select(.key | contains (\"seqno\")) | select(.value == \"${max_seqno}\") .key" | awk -F'/' '{print $(NF-1)}' | sort -n | head -1)
		mysql_note "Found the min ip: (${min_ip}) from the ips with max_seqno"
		mysql_note "The (${min_ip}) should be the bootstraper"
		if [ "${min_ip}" == "${HOSTNAME}" ]; then
			mysql_warn "This node is safe to bootstrap"
			cluster_join=
			break
		else
			mysql_note "This node is not safe to bootstrap"
			sleep 1
		fi
	done

	mysql_note "Cluster_join is: ${cluster_join}"
}

run_galera() {
	mysql_note "step into run_galera"

	mysql_note "Starting reporting script in the background"
	nohup /report_status.sh ${CLUSTER_NAME} ${REPORT_TTL} ${DISCOVERY_SERVICE} &

	mysql_note "Starting mariadbd process"
	GRASTATE=${DATADIR}/grastate.dat
	if [ -z ${cluster_join} ]; then
		export _WSREP_NEW_CLUSTER='--wsrep-new-cluster'
		# set safe_to_bootstrap = 1
		[ -f ${GRASTATE} ] && sed -i "s|safe_to_bootstrap.*|safe_to_bootstrap: 1|g" ${GRASTATE}
	else
		export _WSREP_NEW_CLUSTER=''
	fi
	uuid=$(cat ${TMP} | tr ' ' "\n" | grep -e '[a-z0-9]*-[a-z0-9]*:[-\0-9]' | tail -1 | cut -d ":" -f 1)
	seqno=$(cat ${TMP} | tr ' ' "\n" | grep -e '[a-z0-9]*-[a-z0-9]*:[-\0-9]' | tail -1 | cut -d ":" -f 2)
	[ -f ${GRASTATE} ] && sed -i "s|uuid.*|uuid: ${uuid}|g" ${GRASTATE}
	[ -f ${GRASTATE} ] && sed -i "s|seqno.*|seqno: ${seqno}|g" ${GRASTATE}
	exec mariadbd --wsrep-cluster-name=${CLUSTER_NAME} --wsrep-cluster-address="gcomm://${PEERS}" $_WSREP_NEW_CLUSTER ${CMDARG}
}

rename_root() {
	mysql_note "step into rename root"
	# Rename root account for security reasons
	if [ "${MYSQL_ROOT_USER}" = "root" ]; then
		mysql_note "skip rename root"
	else
		docker_process_sql --database=mysql <<<"RENAME USER 'root'@'localhost' TO '${MYSQL_ROOT_USER}'@'localhost', 'root'@'127.0.0.1' TO '${MYSQL_ROOT_USER}'@'127.0.0.1';"
	fi
}

_main() {
	mysql_note "step into _main"
	# if command starts with an option, prepend mariadbd
	if [ "${1:0:1}" = '-' ]; then
		set -- mariadbd "$@"
	fi

	#wait dns ok
	check_dns

	# skip setup if they aren't running mariadbd or want an option that stops mysqld
	if [ "$1" = 'mariadbd' ] && ! _mysql_want_help "$@"; then
		mysql_note "Entrypoint script for MariaDB Server ${MARIADB_VERSION} started."

		mysql_check_config "$@"
		# Load various environment variables
		docker_setup_env "$@"
		docker_create_db_directories

		# If container is started as root user, restart as dedicated mysql user
		if [ "$(id -u)" = "0" ]; then
			mysql_note "Switching to dedicated user 'mysql'"
			exec gosu mysql "${BASH_SOURCE}" "$@"
		fi

		# set gtid_domain_id
		seq=`echo ${HOSTNAME} | awk -F '-' '{print $NF}'`
		echo "gtid_domain_id = ${seq}" >> /etc/mysql/conf.d/my.cnf

		# there's no database, so it needs to be initialized
		if [ -z "${DATABASE_ALREADY_EXISTS}" ]; then
			docker_verify_minimum_env

			# check dir permissions to reduce likelihood of half-initialized database
			ls /docker-entrypoint-initdb.d/ > /dev/null

			docker_init_database_dir "$@"

			mysql_note "Starting temporary server"
			docker_temp_server_start "$@"
			mysql_note "Temporary server started."

			docker_setup_db
			docker_process_init_files /docker-entrypoint-initdb.d/*
			# Wait until after /docker-entrypoint-initdb.d is performed before setting
			# root@localhost password to a hash we don't know the password for.
			if [ -n "${MARIADB_ROOT_PASSWORD_HASH}" ]; then
				mysql_note "Setting root@localhost password hash"
				docker_process_sql --dont-use-mysql-root-password --binary-mode <<-EOSQL
					SET @@SESSION.SQL_LOG_BIN=0;
					SET PASSWORD FOR 'root'@'localhost'= '${MARIADB_ROOT_PASSWORD_HASH}';
				EOSQL
			fi

			rename_root

			mysql_note "Stopping temporary server"
			docker_temp_server_stop
			mysql_note "Temporary server stopped"

			echo
			mysql_note "MariaDB init process done. Ready for start up."
			echo
		# MDEV-27636 mariadb_upgrade --check-if-upgrade-is-needed cannot be run offline
		#elif mariadb-upgrade --check-if-upgrade-is-needed; then
		elif _check_if_upgrade_is_needed; then
			docker_mariadb_upgrade "$@"
		fi
	fi
	if [ "${CLUSTER_SIZE}" = "1" ]; then
		mysql_note "run mysql without galera..."
		exec mariadbd --wsrep-on=0
	fi
	add_wsrep_conf
	check_safe_bootstrap
	check_force_bootstrap
	get_cluster_join
	run_galera "$@"
}

# If we are sourced from elsewhere, don't perform any further actions
if ! _is_sourced; then
	_main "$@"
fi
