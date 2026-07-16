GET https://fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com/hcmRestApi/resources/latest/recruitingCEJobRequisitionDetails
Accept: application/json
Accept-Language: en
Ora-Irc-Language: en
[QueryStringParams]
onlyData: true
fields: Id,Title,ExternalPostedStartDate,PrimaryLocation,WorkplaceType,ExternalDescriptionStr,CorporateDescriptionStr,ExternalResponsibilitiesStr,ExternalQualificationsStr;secondaryLocations:Name
finder: ById;Id="386920",siteNumber=CX_1

HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.items[0].Id" == "386920"
jsonpath "$.items[0].Title" exists
jsonpath "$.items[0].ExternalDescriptionStr" exists
