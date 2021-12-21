#!/bin/bash -ex

dir_path=$(dirname $(realpath $0))
export PATH=$PATH:$dir_path

#set NOVENV is current environment is not a python virtual env
[ "$VIRTUAL_ENV" != "" ] || NOVENV=1

function install_reqs() {
	python3 $(which pip3) install -r requirements.txt
}

function prepare_venv() {
    echo "Preparing environment"
    virtualenv -p python3 venv && source venv/bin/activate && install_reqs
    echo "Environment ready"
}

function start_mocked_dependencies() {
    python3 $(which pip3) install -r requirements_mocks.txt
    pushd $dir_path/mocks/insights-content-service && uvicorn content_server:app --port 8082 &
    pushd $dir_path/mocks/prometheus && uvicorn push_gateway:app --port 9091 &
    trap 'kill $(lsof -ti:8082); kill $(lsof -ti:9091);' EXIT
    pushd $dir_path
}

[ "$NOVENV" != "1" ] && install_reqs || prepare_venv || exit 1

#launch mocked services if WITHMOCK is provided
[ "$WITHMOCK" == "1" ] && start_mocked_dependencies

PYTHONDONTWRITEBYTECODE=1 python3 "$(which behave)" --tags=-skip -D dump_errors=true @feature_list.txt "$@"
