package differ

// FetchStatusContentError occurs when failing fetching the status content
type FetchStatusContentError struct{}

func (e *FetchStatusContentError) Error() string {
	return "FetchStatusContentEror"
}

// StatusStorageError is related to any storage error
type StatusStorageError struct{}

func (e *StatusStorageError) Error() string {
	return "StatusStorageError"
}

// KafkaBrokerError represent an error related to Kafka initialization
type KafkaBrokerError struct{}

func (e *KafkaBrokerError) Error() string {
	return "KafkaBrokerError"
}

// ServiceLogError represents an error when creating ServiceLog connection
type ServiceLogError struct {
	Msg string
}

func (e *ServiceLogError) Error() string {
	return e.Msg
}

// StatusMetricsError is related to any storage error
type StatusMetricsError struct{}

func (e *StatusMetricsError) Error() string {
	return "StatusMetricsError"
}

// StatusConfiguration is related to any storage error
type StatusConfiguration struct{}

func (e *StatusConfiguration) Error() string {
	return "StatusConfiguration"
}

// StatusEventFilterError is related to any storage error
type StatusEventFilterError struct {
	Msg string
}

func (e *StatusEventFilterError) Error() string {
	return e.Msg
}
