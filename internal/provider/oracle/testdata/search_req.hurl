GET https://fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com/hcmRestApi/resources/latest/recruitingCEJobRequisitions
Accept: application/json
Accept-Language: en
Ora-Irc-Language: en
[QueryStringParams]
onlyData: true
fields: TotalJobsCount,Limit,Offset;requisitionList:Id,Title,PostedDate,PrimaryLocation,WorkplaceType,WorkplaceTypeCode;requisitionList.secondaryLocations:Name;titlesFacet:Id,Name,TotalCount;locationsFacet:Id,Name,TotalCount;categoriesFacet:Id,Name,TotalCount;postingDatesFacet:Id,Name,TotalCount;workLocationsFacet:Id,Name,TotalCount;organizationsFacet:Id,Name,TotalCount;workplaceTypesFacet:Id,Name,TotalCount
finder: findReqs;siteNumber=CX_1,facetsList=NONE,limit=3,offset=0

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.items[0].TotalJobsCount" exists
jsonpath "$.items[0].requisitionList" count == 3
