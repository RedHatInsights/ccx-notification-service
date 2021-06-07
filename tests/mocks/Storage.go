package mocks

import (
	types "github.com/RedHatInsights/ccx-notification-service/types"
	mock "github.com/stretchr/testify/mock"
)

// Storage is a mock type for the Storage type
type Storage struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *Storage) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ReadClusterList provides a mock function with given fields:
func (_m *Storage) ReadClusterList() ([]types.ClusterEntry, error) {
	ret := _m.Called()

	var r0 []types.ClusterEntry
	if rf, ok := ret.Get(0).(func() []types.ClusterEntry); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.ClusterEntry)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReadLastNNotificationRecords provides a mock function with given fields: clusterEntry, numberOfRecords
func (_m *Storage) ReadLastNNotificationRecords(clusterEntry types.ClusterEntry, numberOfRecords int) ([]types.NotificationRecord, error) {
	ret := _m.Called(clusterEntry, numberOfRecords)

	var r0 []types.NotificationRecord
	if rf, ok := ret.Get(0).(func(types.ClusterEntry, int) []types.NotificationRecord); ok {
		r0 = rf(clusterEntry, numberOfRecords)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.NotificationRecord)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(types.ClusterEntry, int) error); ok {
		r1 = rf(clusterEntry, numberOfRecords)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReadNotificationTypes provides a mock function with given fields:
func (_m *Storage) ReadNotificationTypes() ([]types.NotificationType, error) {
	ret := _m.Called()

	var r0 []types.NotificationType
	if rf, ok := ret.Get(0).(func() []types.NotificationType); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.NotificationType)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReadReportForCluster provides a mock function with given fields: orgID, clusterName
func (_m *Storage) ReadReportForCluster(orgID types.OrgID, clusterName types.ClusterName) (types.ClusterReport, types.Timestamp, error) {
	ret := _m.Called(orgID, clusterName)

	var r0 types.ClusterReport
	if rf, ok := ret.Get(0).(func(types.OrgID, types.ClusterName) types.ClusterReport); ok {
		r0 = rf(orgID, clusterName)
	} else {
		r0 = ret.Get(0).(types.ClusterReport)
	}

	var r1 types.Timestamp
	if rf, ok := ret.Get(1).(func(types.OrgID, types.ClusterName) types.Timestamp); ok {
		r1 = rf(orgID, clusterName)
	} else {
		r1 = ret.Get(1).(types.Timestamp)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(types.OrgID, types.ClusterName) error); ok {
		r2 = rf(orgID, clusterName)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ReadReportForClusterAtOffset provides a mock function with given fields: orgID, clusterName, offset
func (_m *Storage) ReadReportForClusterAtOffset(orgID types.OrgID, clusterName types.ClusterName, offset types.KafkaOffset) (types.ClusterReport, error) {
	ret := _m.Called(orgID, clusterName, offset)

	var r0 types.ClusterReport
	if rf, ok := ret.Get(0).(func(types.OrgID, types.ClusterName, types.KafkaOffset) types.ClusterReport); ok {
		r0 = rf(orgID, clusterName, offset)
	} else {
		r0 = ret.Get(0).(types.ClusterReport)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(types.OrgID, types.ClusterName, types.KafkaOffset) error); ok {
		r1 = rf(orgID, clusterName, offset)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReadReportForClusterAtTime provides a mock function with given fields: orgID, clusterName, updatedAt
func (_m *Storage) ReadReportForClusterAtTime(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) (types.ClusterReport, error) {
	ret := _m.Called(orgID, clusterName, updatedAt)

	var r0 types.ClusterReport
	if rf, ok := ret.Get(0).(func(types.OrgID, types.ClusterName, types.Timestamp) types.ClusterReport); ok {
		r0 = rf(orgID, clusterName, updatedAt)
	} else {
		r0 = ret.Get(0).(types.ClusterReport)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(types.OrgID, types.ClusterName, types.Timestamp) error); ok {
		r1 = rf(orgID, clusterName, updatedAt)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReadStates provides a mock function with given fields:
func (_m *Storage) ReadStates() ([]types.State, error) {
	ret := _m.Called()

	var r0 []types.State
	if rf, ok := ret.Get(0).(func() []types.State); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.State)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// WriteNotificationRecord provides a mock function with given fields: notificationRecord
func (_m *Storage) WriteNotificationRecord(notificationRecord types.NotificationRecord) error {
	ret := _m.Called(notificationRecord)

	var r0 error
	if rf, ok := ret.Get(0).(func(types.NotificationRecord) error); ok {
		r0 = rf(notificationRecord)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// WriteNotificationRecordForCluster provides a mock function with given fields: clusterEntry, notificationTypeID, stateID, report, notifiedAt, errorLog
func (_m *Storage) WriteNotificationRecordForCluster(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string) error {
	ret := _m.Called(clusterEntry, notificationTypeID, stateID, report, notifiedAt, errorLog)

	var r0 error
	if rf, ok := ret.Get(0).(func(types.ClusterEntry, types.NotificationTypeID, types.StateID, types.ClusterReport, types.Timestamp, string) error); ok {
		r0 = rf(clusterEntry, notificationTypeID, stateID, report, notifiedAt, errorLog)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// WriteNotificationRecordImpl provides a mock function with given fields: orgID, accountNumber, clusterName, notificationTypeID, stateID, report, updatedAt, notifiedAt, errorLog
func (_m *Storage) WriteNotificationRecordImpl(orgID types.OrgID, accountNumber types.AccountNumber, clusterName types.ClusterName, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, updatedAt types.Timestamp, notifiedAt types.Timestamp, errorLog string) error {
	ret := _m.Called(orgID, accountNumber, clusterName, notificationTypeID, stateID, report, updatedAt, notifiedAt, errorLog)

	var r0 error
	if rf, ok := ret.Get(0).(func(types.OrgID, types.AccountNumber, types.ClusterName, types.NotificationTypeID, types.StateID, types.ClusterReport, types.Timestamp, types.Timestamp, string) error); ok {
		r0 = rf(orgID, accountNumber, clusterName, notificationTypeID, stateID, report, updatedAt, notifiedAt, errorLog)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CleanupForOrganization is just a stub for a proper mock method
func (_m *Storage) CleanupForOrganization(orgID types.OrgID, maxAge string, statement string) (int, error) {
	return 0, nil
}

// CleanupNewReportsForOrganization is just a stub for a proper mock method
func (_m *Storage) CleanupNewReportsForOrganization(orgID types.OrgID, maxAge string) (int, error) {
	return 0, nil
}

// CleanupOldReportsForOrganization is just a stub for a proper mock method
func (_m *Storage) CleanupOldReportsForOrganization(orgID types.OrgID, maxAge string) (int, error) {
	return 0, nil
}
