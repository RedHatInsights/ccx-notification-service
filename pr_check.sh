#!/bin/bash

# --------------------------------------------
# Options that must be configured by app owner
# --------------------------------------------
APP_NAME="ccx-data-pipeline"  # name of app-sre "application" folder this component lives in
COMPONENT_NAME="ccx-notification-service"  # name of app-sre "resourceTemplate" in deploy.yaml for this component
IMAGE="quay.io/cloudservices/ccx-notification-service"
COMPONENTS="ccx-notification-writer ccx-notification-service ccx-notification-db-cleaner"  # space-separated list of components to laod
COMPONENTS_W_RESOURCES="ccx-notification-service"  # component to keep
CACHE_FROM_LATEST_IMAGE="true"

export IQE_PLUGINS="ccx"
export IQE_MARKER_EXPRESSION=""
# Workaround: There are no cleaner specific integration tests. Check that the service loads and iqe plugin works.
export IQE_FILTER_EXPRESSION="test_plugin_accessible"
export IQE_REQUIREMENTS_PRIORITY=""
export IQE_TEST_IMPORTANCE=""
export IQE_CJI_TIMEOUT="30m"


function build_image() {
    source $CICD_ROOT/build.sh
}

function deploy_ephemeral() {
    source $CICD_ROOT/deploy_ephemeral_env.sh
}

function run_smoke_tests() {
    source $CICD_ROOT/cji_smoke_test.sh
    source $CICD_ROOT/post_test_results.sh  # publish results in Ibutsu
}


# Install bonfire repo/initialize
CICD_URL=https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd
curl -s $CICD_URL/bootstrap.sh > .cicd_bootstrap.sh && source .cicd_bootstrap.sh
echo "creating PR image"
build_image

echo "deploying to ephemeral"
deploy_ephemeral

echo "running PR smoke tests"
run_smoke_tests
