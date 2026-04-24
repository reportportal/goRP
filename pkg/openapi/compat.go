package openapi

// Short-name type aliases that preserve the previous public API surface.
//
// The openapi package was regenerated from a newer ReportPortal API spec
// (develop-531) which maps every Java class to its fully-qualified Go name.
// These aliases let existing code in the gorp layer continue to compile
// without changes.

type (
	// Reporting types

	StartLaunchRQ       = ComEpamReportportalBaseReportingStartLaunchRQ
	EntryCreatedAsyncRS = ComEpamReportportalBaseReportingEntryCreatedAsyncRS
	FinishExecutionRQ   = ComEpamReportportalBaseReportingFinishExecutionRQ
	FinishLaunchRS      = ComEpamReportportalBaseModelLaunchFinishLaunchRS

	OperationCompletionRS = ComEpamReportportalBaseReportingOperationCompletionRS

	StartTestItemRQ  = ComEpamReportportalBaseReportingStartTestItemRQ
	FinishTestItemRQ = ComEpamReportportalBaseReportingFinishTestItemRQ

	SaveLogRQ = ComEpamReportportalBaseReportingSaveLogRQ
	File      = ComEpamReportportalBaseReportingSaveLogRQFile

	// Launch types

	LaunchResource     = ComEpamReportportalBaseReportingLaunchResource
	PageLaunchResource = ComEpamReportportalBaseModelPageComEpamReportportalBaseReportingLaunchResource

	// Filter types

	UserFilterResource     = ComEpamReportportalBaseModelFilterUserFilterResource
	PageUserFilterResource = ComEpamReportportalBaseModelPageComEpamReportportalBaseModelFilterUserFilterResource
	UserFilterCondition    = ComEpamReportportalBaseModelFilterUserFilterCondition
	Order                  = ComEpamReportportalBaseModelFilterOrder

	// Metadata

	PageMetadata = ComEpamReportportalBaseModelPagePageMetadata

	// Merge

	MergeLaunchesRQ = ComEpamReportportalBaseReportingMergeLaunchesRQ

	// Attributes

	ItemAttributesRQ = ComEpamReportportalBaseReportingItemAttributesRQ
)

// NewPageLaunchResource is a constructor alias for backwards compatibility.
func NewPageLaunchResource() *PageLaunchResource {
	return NewComEpamReportportalBaseModelPageComEpamReportportalBaseReportingLaunchResource()
}

// MergeLaunchesRQ is a method alias on ApiMergeLaunchesOldUuidRequest (v2 async endpoint)
// so callers can use the short name instead of ComEpamReportportalBaseReportingMergeLaunchesRQ.
func (r ApiMergeLaunchesOldUuidRequest) MergeLaunchesRQ(
	rq ComEpamReportportalBaseReportingMergeLaunchesRQ,
) ApiMergeLaunchesOldUuidRequest {
	return r.ComEpamReportportalBaseReportingMergeLaunchesRQ(rq)
}

// MergeLaunchesRQ is a method alias on ApiMergeLaunchesOldUuid1Request (v1 sync endpoint).
func (r ApiMergeLaunchesOldUuid1Request) MergeLaunchesRQ(
	rq ComEpamReportportalBaseReportingMergeLaunchesRQ,
) ApiMergeLaunchesOldUuid1Request {
	return r.ComEpamReportportalBaseReportingMergeLaunchesRQ(rq)
}
