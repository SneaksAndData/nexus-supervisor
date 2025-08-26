#!/usr/bin/env bash

cqlsh localhost -e "CREATE KEYSPACE nexus WITH replication = { 'class': 'SimpleStrategy', 'replication_factor': 1 };"

echo 'Applying checkpoints table'

cqlsh localhost -f /opt/storage/checkpoints.cql

echo 'Checking table'

cqlsh localhost -e 'SELECT * FROM nexus.checkpoints'
