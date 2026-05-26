#!/bin/bash

# Copyright 2026 The Kubernetes Authors.
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

# Common E2E test skip regex to avoid duplication across periodic scripts.
export KOPS_SKIP_REGEX='\[Slow\]|\[Serial\]|\[Disruptive\]|\[Flaky\]|\[Feature:.+\]|\[Driver: nfs\]|\[Driver: nfs3\]|NFS|Flexvolumes|Services should function for service endpoints using hostNetwork|RuntimeClass should run a Pod requesting a RuntimeClass with scheduling without taints'
