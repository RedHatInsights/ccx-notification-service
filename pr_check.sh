#!/bin/bash

# --------------------------------------------
# Options that must be configured by app owner
# --------------------------------------------
APP_NAME="ccx-data-pipeline"  # name of app-sre "application" folder this component lives in
COMPONENT_NAME="ccx-notification-service"  # name of app-sre "resourceTemplate" in deploy.yaml for this component
IMAGE="quay.io/cloudservices/ccx-notification-service"
COMPONENTS="ccx-data-pipeline ccx-insights-results dvo-writer ccx-smart-proxy ccx-notification-writer ccx-notification-service ccx-notification-db-cleaner notifications-backend notifications-aggregator notifications-engine insights-content-service ccx-mock-ams ccx-upgrades-sso-mock insights-content-template-renderer ccx-redis"  # space-separated list of components to load
COMPONENTS_W_RESOURCES="ccx-notification-service"  # component to keep
CACHE_FROM_LATEST_IMAGE="true"
DEPLOY_FRONTENDS="false"


export IQE_PLUGINS="ccx"
export IQE_MARKER_EXPRESSION="notifications or servicelog"
export IQE_FILTER_EXPRESSION=""
export IQE_REQUIREMENTS_PRIORITY=""
export IQE_TEST_IMPORTANCE=""
export IQE_CJI_TIMEOUT="30m"
export IQE_SELENIUM="false"
export IQE_ENV="ephemeral"
export IQE_ENV_VARS="DYNACONF_USER_PROVIDER__rbac_enabled=false"


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
