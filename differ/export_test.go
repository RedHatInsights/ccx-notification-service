/*
Copyright © 2020, 2021, 2022, 2023 Red Hat, Inc.

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

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/export_test.html

// Export for testing
//
// This source file contains name aliases of all package-private functions
// that need to be called from unit tests. Aliases should start with uppercase
// letter because unit tests belong to different package.
//
// Please look into the following blogpost:
// https://medium.com/@robiplus/golang-trick-export-for-test-aa16cbd7b8cd
// to see why this trick is needed.
var (
	NotificationType  = notificationType
	NotificationTypes = &notificationTypes
	States            = &states

	GetAllContentFromMap               = getAllContentFromMap
	ConvertLogLevel                    = convertLogLevel
	PushMetrics                        = pushMetrics
	GenerateInstantNotificationMessage = generateInstantNotificationMessage
	GenerateNotificationPayloadURL     = generateNotificationPayloadURL
	AppendEventToNotificationMessage   = appendEventToNotificationMessage
	CalculateTotalRisk                 = calculateTotalRisk
	ModuleToRuleName                   = moduleToRuleName
	AssertNotificationDestination      = assertNotificationDestination

	CheckArgs            = checkArgs
	GetState             = getState
	GetStates            = getStates
	GetNotificationType  = getNotificationType
	GetNotificationTypes = getNotificationTypes
)

const (
	VersionMessage              = versionMessage
	AuthorsMessage              = authorsMessage
	NotificationTypeInstant     = notificationTypeInstant
	NotificationBundleName      = notificationBundleName
	NotificationApplicationName = notificationApplicationName
	NotificationPayloadRuleURL  = notificationPayloadRuleURL
	NotificationStateSent       = notificationStateSent
	NotificationStateSame       = notificationStateSame
	NotificationStateLower      = notificationStateLower
	NotificationStateError      = notificationStateError
)

func InClauseFromStringSlice(slice []string) string {
	return inClauseFromStringSlice(slice)
}
