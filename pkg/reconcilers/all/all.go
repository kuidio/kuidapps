/*
Copyright 2024 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package all

import (
	_ "github.com/kuidio/kuidapps/pkg/reconcilers/link"
	_ "github.com/kuidio/kuidapps/pkg/reconcilers/network"
	_ "github.com/kuidio/kuidapps/pkg/reconcilers/networkdesign"
	_ "github.com/kuidio/kuidapps/pkg/reconcilers/networkdevice"
	_ "github.com/kuidio/kuidapps/pkg/reconcilers/networkpackage"
	_ "github.com/kuidio/kuidapps/pkg/reconcilers/networkparams"
	_ "github.com/kuidio/kuidapps/pkg/reconcilers/topology"
)
