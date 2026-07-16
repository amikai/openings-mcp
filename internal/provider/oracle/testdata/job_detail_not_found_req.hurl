GET https://fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com/hcmRestApi/resources/latest/recruitingCEJobRequisitionDetails
Accept: application/json
Accept-Language: en
Ora-Irc-Language: en
[QueryStringParams]
onlyData: true
fields: Id,Title,ExternalPostedStartDate,PrimaryLocation,WorkplaceType,ExternalDescriptionStr,CorporateDescriptionStr,ExternalResponsibilitiesStr,ExternalQualificationsStr;secondaryLocations:Name
finder: ById;Id="999999999999",siteNumber=CX_1

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.items" count == 0
