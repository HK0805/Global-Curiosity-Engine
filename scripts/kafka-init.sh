#!/bin/sh
set -eu

until kafka-topics --bootstrap-server kafka:29092 --list >/dev/null 2>&1; do
  echo "waiting for kafka"
  sleep 2
done

kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic raw.wikipedia.edits --partitions 1 --replication-factor 1
kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic raw.reddit.posts --partitions 1 --replication-factor 1
kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic raw.hn.stories --partitions 1 --replication-factor 1
kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic raw.github.events --partitions 1 --replication-factor 1
kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic raw.gdelt.events --partitions 1 --replication-factor 1
kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic normalized.events --partitions 1 --replication-factor 1
kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic topics.extracted --partitions 1 --replication-factor 1
kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic topics.metrics --partitions 1 --replication-factor 1
kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic topics.scored --partitions 1 --replication-factor 1
kafka-topics --bootstrap-server kafka:29092 --create --if-not-exists --topic topics.spikes --partitions 1 --replication-factor 1
