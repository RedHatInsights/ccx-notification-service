/*
Copyright © 2022 Red Hat, Inc.

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

package differ

import "strings"

// inClauseFromStringSlice is a helper function to construct `in` clause for SQL
// statement from a given slice of string items. If the slice is empty, an
// empty string will be returned, making the in clause fail.
func inClauseFromStringSlice(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	return "'" + strings.Join(slice, `','`) + `'`
}
