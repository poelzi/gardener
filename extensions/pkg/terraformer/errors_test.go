// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package terraformer

import (
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Errors", func() {
	Describe("#retrieveTerraformErrors", func() {
		var (
			errorLog1error1 = `Error waiting to create Router: Error waiting for Creating Router: Quota 'ROUTERS' exceeded.  Limit: 20.0 globally.

  on tf/main.tf line 32, in resource "google_compute_router" "router":
  32: resource "google_compute_router" "router"{`
			errorLog1 = `foo bar
foo
bar
foo

Error: ` + errorLog1error1 + `

`

			errorLog2error1 = `Error creating service account: googleapi: Error 409: Service account shoot--foo--bar already exists within project projects/my-project., alreadyExists

  on tf/main.tf line 11, in resource "google_service_account" "serviceaccount":
  11: resource "google_service_account" "serviceaccount" {`
			errorLog2error2 = `Error creating Network: googleapi: Error 409: The resource 'projects/my-project/global/networks/shoot--foo--bar' already exists, alreadyExists

  on tf/main.tf line 20, in resource "google_compute_network" "network":
  20: resource "google_compute_network" "network" {`
			errorLog2 = `
Initializing the backend...

Initializing provider plugins...

The following providers do not have any version constraints in configuration,
so the latest version was installed.

To prevent automatic upgrades to new major versions that may contain breaking
changes, it is recommended to add version = "..." constraints to the
corresponding provider blocks in configuration, with the constraint strings
suggested below.

* provider.google: version = "~> 2.14"
* provider.null: version = "~> 2.1"

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
null_resource.outputs: Refreshing state... [id=1234]
google_service_account.serviceaccount: Creating...
google_compute_network.network: Creating...

Error: ` + errorLog2error1 + `

Error: ` + errorLog2error2 + `

Nothing to do.
			`

			errorLog3error1 = `Error creating IAM Role shoot--foo--bar-bastions: EntityAlreadyExists: Role with name shoot--foo--bar-bastions already exists.
\tstatus code: 409, request id: d9e4221c-d488-4e52-98a9-a2d53a10b0fd

  on tf/main.tf line 280, in resource "aws_iam_role" "bastions":
 280: resource "aws_iam_role" "bastions" {`
			errorLog3error2 = `Error creating IAM Role shoot--foo--bar-nodes: EntityAlreadyExists: Role with name shoot--foo--bar-nodes already exists.
\tstatus code: 409, request id: fb991e24-8a9c-4d92-b613-4ff1c7e7a17c

  on tf/main.tf line 327, in resource "aws_iam_role" "nodes":
 327: resource "aws_iam_role" "nodes" {`
			errorLog3error3 = `Error import KeyPair: InvalidKeyPair.Duplicate: The keypair 'shoot--foo--bar-ssh-publickey' already exists.
\tstatus code: 400, request id: c5df52d5-aca6-459f-8004-1f3dd49a085e

  on tf/main.tf line 393, in resource "aws_key_pair" "kubernetes":
 393: resource "aws_key_pair" "kubernetes" {`
			errorLog3 = `Error: ` + errorLog3error1 + `

Error: ` + errorLog3error2 + `

Error: ` + errorLog3error3 + `
`

			errorLog4error1 = `Unable to list provider registration status, it is possible that this is due to invalid credentials or the service principal does not have permission to use the Resource Manager API, Azure error: azure.BearerAuthorizer#WithAuthorization: Failed to refresh the Token for request to https://management.azure.com/subscriptions/7021843c-b121-46f3-91a3-9cdd0e0f415b/providers?api-version=2016-02-01: StatusCode=401 -- Original Error: adal: Refresh request failed. Status Code = '401'. Response body: {"error":"invalid_client","error_description":"AADSTS7000222: The provided client secret keys are expired.\r\nTrace ID: a586af20-bd59-4bd7-8c85-443558347400\r\nCorrelation ID: a4b83fcf-5fd9-44ea-9dbc-43abb1d59a75\r\nTimestamp: 2019-10-31 12:37:32Z","error_codes":[7000222],"timestamp":"2019-10-31 12:37:32Z","trace_id":"a586af20-bd59-4bd7-8c85-443558347400","correlation_id":"a4b83fcf-5fd9-44ea-9dbc-43abb1d59a75"}

on tf/main.tf line 1, in provider "azurerm":
 1: provider "azurerm" {`

			errorLog4 = `

Error: ` + errorLog4error1 + `

Error: ` + errorLog4error1 + `
`

			errorLog5error1 = `Error creating VPC: VpcLimitExceeded: The maximum number of VPCs has been reached.
status code: 400, request id: bc36adce-333c-4ddc-a336-12494ac8cca4

on tf/main.tf line 21, in resource "aws_vpc" "vpc":
21: resource "aws_vpc" "vpc" {`

			errorLog5error2 = `Error creating EIP: AddressLimitExceeded: The maximum number of addresses has been reached.
status code: 400, request id: f6a78181-00ad-4a62-911f-dda604041548

on tf/main.tf line 226, in resource "aws_eip" "eip_natgw_z0":
226: resource "aws_eip" "eip_natgw_z0" {`

			errorLog5 = `aws_eip.eip_natgw_z0: Creating...

Error: ` + errorLog5error1 + `



Error: ` + errorLog5error2

			regexUUID         = regexp.MustCompile(`(?i)[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}`)
			regexMultiNewline = regexp.MustCompile(`\n{2,}`)
		)

		DescribeTable("detects correct errors",
			func(podName, logs, expectedMessage string) {
				Expect(retrieveTerraformErrors(podName, logs)).To(ConsistOf(expectedMessage))
			},

			Entry("pod1", "pod1", errorLog1, "-> Pod 'pod1' reported:\n* "+regexUUID.ReplaceAllString(regexMultiNewline.ReplaceAllString(errorLog1error1, "\n"), "<omitted>")),
			Entry("pod2", "pod2", errorLog2, "-> Pod 'pod2' reported:\n* "+regexUUID.ReplaceAllString(regexMultiNewline.ReplaceAllString(errorLog2error2, "\n")+"\n* "+regexMultiNewline.ReplaceAllString(errorLog2error1, "\n"), "<omitted>")),
			Entry("pod3", "pod3", errorLog3, "-> Pod 'pod3' reported:\n* "+regexUUID.ReplaceAllString(regexMultiNewline.ReplaceAllString(errorLog3error1, "\n")+"\n* "+regexMultiNewline.ReplaceAllString(errorLog3error2, "\n")+"\n* "+regexMultiNewline.ReplaceAllString(errorLog3error3, "\n"), "<omitted>")),
			Entry("pod4", "pod4", errorLog4, "-> Pod 'pod4' reported:\n* "+regexUUID.ReplaceAllString(regexMultiNewline.ReplaceAllString(errorLog4error1, "\n")+"\n* "+regexMultiNewline.ReplaceAllString(errorLog4error1, "\n"), "<omitted>")),
			Entry("pod5", "pod5", errorLog5, "-> Pod 'pod5' reported:\n* "+regexUUID.ReplaceAllString(regexMultiNewline.ReplaceAllString(errorLog5error2, "\n")+"\n* "+regexMultiNewline.ReplaceAllString(errorLog5error1, "\n"), "<omitted>")),
		)
	})
})
