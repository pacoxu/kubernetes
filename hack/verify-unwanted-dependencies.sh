#!/usr/bin/env bash

# Copyright 2021 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script checks unwanted dependencies. It checks if any items in the
# "unwanted but still present" list(.unwanted_dependencies) are no longer present 
# and prompts to move them to the "absent and not allowed" list(.not_allowed_dependencies)
# It also checks if any items in the "absent and not allowed" list are present and fails.
# Usage: `hack/verify-unwanted-dependencies.sh`.

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${KUBE_ROOT}/hack/lib/init.sh"

# We store unwanted dependencies which is still present in .unwanted_dependencies
unwanted_dependencies_file="${KUBE_ROOT}/hack/.unwanted_dependencies"

# We store not-allowed dependencies in .not_allowed_dependencies
not_allowed_dependencies_file="${KUBE_ROOT}/hack/.not_allowed_dependencies"

# Ensure that file is sorted.
kube::util::check-file-in-alphabetical-order "${unwanted_dependencies_file}"
kube::util::check-file-in-alphabetical-order "${not_allowed_dependencies_file}"

unwanted_dependencies=()
while IFS='' read -r line; do
  unwanted_dependencies+=("$KUBE_ROOT/$line")
done < <(cat "${unwanted_dependencies_file}")

