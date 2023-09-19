#!/bin/bash

set -ex

MACKEREL_APIKEY=`tfstate-lookup --state .terraform/terraform.tfstate aws_ssm_parameter.mackerel_apikey.value`
cat <<EOF | mkr throw --service prepalert
manual.trigger.value 0 `date -v -2M +%s`
manual.trigger.value 1 `date -v -1M +%s`
manual.trigger.value 1 `date +%s`
EOF
sleep 60
cat <<EOF | mkr throw --service prepalert
manual.trigger.value 0 `date +%s`
EOF
