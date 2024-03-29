#!/usr/bin/env bash
# Copyright 2022 Red Hat, Inc
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

STORAGE=$1

function run_profiler() {
    # shellcheck disable=SC2046
    if ! go test -v -timeout 2m -cpuprofile cpuprofile.prof github.com/RedHatInsights/ccx-notification-service/differ
    then
        echo "profiling failed"
        exit 1
    fi

    if ! go test -v -timeout 2m -memprofile memprofile.prof github.com/RedHatInsights/ccx-notification-service/differ
    then
        echo "profiling failed"
        exit 1
    fi

}

function show_results() {
    echo "Memory profile"
    go tool pprof -top memprofile.prof

    echo "CPU profile"
    go tool pprof -top cpuprofile.prof
}

run_profiler
show_results
