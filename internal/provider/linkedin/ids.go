package linkedin

// WorkplaceTypeIDs maps a human label to its f_WT value.
var WorkplaceTypeIDs = map[string]string{
	"On-site": WorkplaceOnSite,
	"Remote":  WorkplaceRemote,
	"Hybrid":  WorkplaceHybrid,
}

// JobTypeIDs maps a human label to its f_JT value.
var JobTypeIDs = map[string]string{
	"Full-time":  JobTypeFullTime,
	"Part-time":  JobTypePartTime,
	"Contract":   JobTypeContract,
	"Temporary":  JobTypeTemporary,
	"Internship": JobTypeInternship,
}
