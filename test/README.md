# Integration Tests

Can only be executed locally as on-prem artifactory do not have the worker service.

## Running Integration Tests

   1. Setup a JPD
   1. Create an identity token
   1. Run the tests with the following environments variables
      * **JF_PLATFORM_URL**: the JPD url
      * **JF_PLATFORM_ACCESS_TOKEN**: the JPD access token