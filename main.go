package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// JSONOutput structures
type Metadata struct {
	GeneratedAt    string `json:"generated_at"`
	Profile        string `json:"profile"`
	CurrentAccount string `json:"current_account"`
	CurrentArn     string `json:"current_arn"`
}

type PolicyDocument struct {
	Version   string        `json:"Version,omitempty"`
	Statement []interface{} `json:"Statement,omitempty"`
	Type      string        `json:"Type,omitempty"`
	Note      string        `json:"Note,omitempty"`
	Error     string        `json:"Error,omitempty"`
}

type AttachedPolicy struct {
	PolicyName     string         `json:"PolicyName"`
	PolicyArn      string         `json:"PolicyArn"`
	PolicyDocument PolicyDocument `json:"PolicyDocument"`
}

type User struct {
	UserName         string           `json:"UserName"`
	Arn              string           `json:"Arn"`
	CreateDate       string           `json:"CreateDate"`
	AttachedPolicies []AttachedPolicy `json:"AttachedPolicies"`
}

type GroupUser struct {
	UserName string `json:"UserName"`
	Arn      string `json:"Arn"`
}

type Group struct {
	GroupName        string           `json:"GroupName"`
	Arn              string           `json:"Arn"`
	CreateDate       string           `json:"CreateDate"`
	Users            []GroupUser      `json:"Users"`
	AttachedPolicies []AttachedPolicy `json:"AttachedPolicies"`
}

type Role struct {
	RoleName         string           `json:"RoleName"`
	Arn              string           `json:"Arn"`
	CreateDate       string           `json:"CreateDate"`
	AttachedPolicies []AttachedPolicy `json:"AttachedPolicies"`
}

type BucketPolicy struct {
	PolicyName     string         `json:"PolicyName"`
	PolicyDocument PolicyDocument `json:"PolicyDocument"`
}

type ObjectLockConfiguration struct {
	Enabled   bool   `json:"Enabled"`
	Mode      string `json:"Mode,omitempty"`
	Retention string `json:"Retention,omitempty"`
	Validity  string `json:"Validity,omitempty"`
}

type Bucket struct {
	BucketName              string                   `json:"BucketName"`
	Arn                     string                   `json:"Arn"`
	Region                  string                   `json:"Region"`
	Policies                []BucketPolicy           `json:"Policies"`
	ObjectLockConfiguration *ObjectLockConfiguration `json:"ObjectLockConfiguration,omitempty"`
	VersioningEnabled       bool                     `json:"VersioningEnabled"`
	PublicAccessBlock       *PublicAccessBlock       `json:"PublicAccessBlock,omitempty"`
}

type PublicAccessBlock struct {
	BlockPublicAcls       bool `json:"BlockPublicAcls"`
	IgnorePublicAcls      bool `json:"IgnorePublicAcls"`
	BlockPublicPolicy     bool `json:"BlockPublicPolicy"`
	RestrictPublicBuckets bool `json:"RestrictPublicBuckets"`
}

type Resources struct {
	Users   []User   `json:"users"`
	Groups  []Group  `json:"groups"`
	Roles   []Role   `json:"roles"`
	Buckets []Bucket `json:"buckets"`
}

type Account struct {
	AccountID     string    `json:"account_id"`
	AccountName   string    `json:"account_name"`
	AccountStatus string    `json:"account_status"`
	Resources     Resources `json:"resources"`
}

type Output struct {
	Metadata Metadata  `json:"metadata"`
	Accounts []Account `json:"accounts"`
}

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	fmt.Printf("aws-iam-audit version %s (commit: %s, built: %s)\n", version, commit, date)
	profileFlag := flag.String("profile", "", "AWS profile to use (required)")
	flag.Parse()

	if *profileFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: --profile is required\n")
		os.Exit(1)
	}

	log.Printf("Using profile: %s\n", *profileFlag)

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithSharedConfigProfile(*profileFlag),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create clients
	stsClient := sts.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)
	orgsClient := organizations.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)

	// Get current account
	log.Println("Retrieving current account...")
	identity, err := stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting caller identity: %v\n", err)
		os.Exit(1)
	}

	currentAccount := *identity.Account
	currentArn := *identity.Arn
	log.Printf("Current account: %s (ARN: %s)\n", currentAccount, currentArn)

	// Try to get organization accounts
	log.Println("Attempting to retrieve Organization accounts...")
	var accountsList []Account

	orgsOutput, err := orgsClient.ListAccounts(context.Background(), &organizations.ListAccountsInput{})
	if err != nil {
		log.Println("Access to Organizations denied, using current account only")
		accountsList = []Account{
			{
				AccountID:     currentAccount,
				AccountName:   "current-account",
				AccountStatus: "ACTIVE",
			},
		}
	} else {
		log.Printf("Access to Organizations OK, %d account(s) found\n", len(orgsOutput.Accounts))
		for _, acc := range orgsOutput.Accounts {
			accountsList = append(accountsList, Account{
				AccountID:     *acc.Id,
				AccountName:   *acc.Name,
				AccountStatus: string(acc.Status),
			})
		}
	}

	// Process accounts
	log.Println("Processing accounts and resources...")
	output := Output{
		Metadata: Metadata{
			GeneratedAt:    time.Now().UTC().Format("2006-01-02T15:04:05Z"),
			Profile:        *profileFlag,
			CurrentAccount: currentAccount,
			CurrentArn:     currentArn,
		},
		Accounts: []Account{},
	}

	for _, account := range accountsList {
		log.Printf("  - Account: %s (%s)\n", account.AccountID, account.AccountName)

		log.Println("    Retrieving users...")
		users, err := getUsers(iamClient)
		if err != nil {
			log.Printf("Error getting users: %v\n", err)
			users = []User{}
		}

		log.Println("    Retrieving groups...")
		groups, err := getGroups(iamClient)
		if err != nil {
			log.Printf("Error getting groups: %v\n", err)
			groups = []Group{}
		}

		log.Println("    Retrieving roles...")
		roles, err := getRoles(iamClient)
		if err != nil {
			log.Printf("Error getting roles: %v\n", err)
			roles = []Role{}
		}

		log.Println("    Retrieving buckets...")
		buckets, err := getBuckets(s3Client)
		if err != nil {
			log.Printf("Error getting buckets: %v\n", err)
			buckets = []Bucket{}
		}

		account.Resources = Resources{
			Users:   users,
			Groups:  groups,
			Roles:   roles,
			Buckets: buckets,
		}

		output.Accounts = append(output.Accounts, account)
	}

	// Output JSON to stdout
	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonBytes))
}

func getUsers(client *iam.Client) ([]User, error) {
	var users []User
	paginator := iam.NewListUsersPaginator(client, &iam.ListUsersInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}

		for _, u := range page.Users {
			user := User{
				UserName:         *u.UserName,
				Arn:              *u.Arn,
				CreateDate:       u.CreateDate.Format(time.RFC3339),
				AttachedPolicies: []AttachedPolicy{},
			}

			log.Printf("      - User: %s\n", user.UserName)

			// Get attached policies
			policies, err := getAttachedUserPolicies(client, user.UserName)
			if err == nil {
				user.AttachedPolicies = policies
			}

			users = append(users, user)
		}
	}

	return users, nil
}

func getGroups(client *iam.Client) ([]Group, error) {
	var groups []Group
	paginator := iam.NewListGroupsPaginator(client, &iam.ListGroupsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}

		for _, g := range page.Groups {
			group := Group{
				GroupName:        *g.GroupName,
				Arn:              *g.Arn,
				CreateDate:       g.CreateDate.Format(time.RFC3339),
				Users:            []GroupUser{},
				AttachedPolicies: []AttachedPolicy{},
			}

			log.Printf("    Group: %s\n", group.GroupName)

			// Get users in group
			usersInGroup, err := getUsersInGroup(client, group.GroupName)
			if err == nil {
				group.Users = usersInGroup
				for _, u := range usersInGroup {
					log.Printf("      - User: %s\n", u.UserName)
				}
			}

			// Get attached policies
			policies, err := getAttachedGroupPolicies(client, group.GroupName)
			if err == nil {
				group.AttachedPolicies = policies
				for _, p := range policies {
					log.Printf("      - Policy: %s\n", p.PolicyName)
				}
			}

			groups = append(groups, group)
		}
	}

	return groups, nil
}

func getRoles(client *iam.Client) ([]Role, error) {
	var roles []Role
	paginator := iam.NewListRolesPaginator(client, &iam.ListRolesInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}

		for _, r := range page.Roles {
			role := Role{
				RoleName:         *r.RoleName,
				Arn:              *r.Arn,
				CreateDate:       r.CreateDate.Format(time.RFC3339),
				AttachedPolicies: []AttachedPolicy{},
			}

			log.Printf("    Role: %s\n", role.RoleName)

			// Get attached policies
			policies, err := getAttachedRolePolicies(client, role.RoleName)
			if err == nil {
				role.AttachedPolicies = policies
				for _, p := range policies {
					log.Printf("      - Policy: %s\n", p.PolicyName)
				}
			}

			roles = append(roles, role)
		}
	}

	return roles, nil
}

func getAttachedUserPolicies(client *iam.Client, userName string) ([]AttachedPolicy, error) {
	var policies []AttachedPolicy
	paginator := iam.NewListAttachedUserPoliciesPaginator(client, &iam.ListAttachedUserPoliciesInput{
		UserName: aws.String(userName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}

		for _, p := range page.AttachedPolicies {
			policyName := *p.PolicyName
			policyArn := *p.PolicyArn
			log.Printf("      - Policy user: %s\n", policyName)

			policy := AttachedPolicy{
				PolicyName: policyName,
				PolicyArn:  policyArn,
			}

			policy.PolicyDocument = getPolicyContent(client, policyArn)
			policies = append(policies, policy)
		}
	}

	return policies, nil
}

func getAttachedGroupPolicies(client *iam.Client, groupName string) ([]AttachedPolicy, error) {
	var policies []AttachedPolicy
	paginator := iam.NewListAttachedGroupPoliciesPaginator(client, &iam.ListAttachedGroupPoliciesInput{
		GroupName: aws.String(groupName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}

		for _, p := range page.AttachedPolicies {
			policyName := *p.PolicyName
			policyArn := *p.PolicyArn

			policy := AttachedPolicy{
				PolicyName: policyName,
				PolicyArn:  policyArn,
			}

			policy.PolicyDocument = getPolicyContent(client, policyArn)
			policies = append(policies, policy)
		}
	}

	return policies, nil
}

func getAttachedRolePolicies(client *iam.Client, roleName string) ([]AttachedPolicy, error) {
	var policies []AttachedPolicy
	paginator := iam.NewListAttachedRolePoliciesPaginator(client, &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}

		for _, p := range page.AttachedPolicies {
			policyName := *p.PolicyName
			policyArn := *p.PolicyArn

			policy := AttachedPolicy{
				PolicyName: policyName,
				PolicyArn:  policyArn,
			}

			policy.PolicyDocument = getPolicyContent(client, policyArn)
			policies = append(policies, policy)
		}
	}

	return policies, nil
}

func getUsersInGroup(client *iam.Client, groupName string) ([]GroupUser, error) {
	var users []GroupUser
	paginator := iam.NewGetGroupPaginator(client, &iam.GetGroupInput{
		GroupName: aws.String(groupName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}

		for _, u := range page.Users {
			users = append(users, GroupUser{
				UserName: *u.UserName,
				Arn:      *u.Arn,
			})
		}
	}

	return users, nil
}

func getPolicyContent(client *iam.Client, policyArn string) PolicyDocument {
	// AWS managed policies
	if strings.HasPrefix(policyArn, "arn:aws:iam::aws:") {
		return PolicyDocument{
			Type: "AWS_MANAGED",
			Note: "Use AWS documentation for details",
		}
	}

	// Customer managed policy
	policyOutput, err := client.GetPolicy(context.Background(), &iam.GetPolicyInput{
		PolicyArn: aws.String(policyArn),
	})
	if err != nil {
		log.Printf("        Error getting policy %s: %v\n", policyArn, err)
		return PolicyDocument{
			Type:  "CUSTOMER_MANAGED",
			Error: fmt.Sprintf("Unable to retrieve: %v", err),
		}
	}

	versionID := policyOutput.Policy.DefaultVersionId
	versionOutput, err := client.GetPolicyVersion(context.Background(), &iam.GetPolicyVersionInput{
		PolicyArn: aws.String(policyArn),
		VersionId: versionID,
	})
	if err != nil {
		log.Printf("        Error getting policy version %s: %v\n", policyArn, err)
		return PolicyDocument{
			Type:  "CUSTOMER_MANAGED",
			Error: fmt.Sprintf("Unable to retrieve version: %v", err),
		}
	}

	// Le document est URL-encodé, il faut le décoder
	decodedDoc, err := url.QueryUnescape(*versionOutput.PolicyVersion.Document)
	if err != nil {
		log.Printf("        Error decoding policy document %s: %v\n", policyArn, err)
		return PolicyDocument{
			Type:  "CUSTOMER_MANAGED",
			Error: fmt.Sprintf("Unable to decode: %v", err),
		}
	}

	// Parse the policy document
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(decodedDoc), &doc); err != nil {
		log.Printf("        Error parsing policy document %s: %v\n", policyArn, err)
		log.Printf("        Document content: %s\n", decodedDoc)
		return PolicyDocument{
			Type:  "CUSTOMER_MANAGED",
			Error: fmt.Sprintf("Unable to parse: %v", err),
		}
	}

	result := PolicyDocument{}

	// Vérifier si Version existe et est une chaîne
	if version, ok := doc["Version"].(string); ok {
		result.Version = version
	}

	// Vérifier si Statement existe
	if statements, ok := doc["Statement"]; ok {
		if stmtArray, ok := statements.([]interface{}); ok {
			result.Statement = stmtArray
		}
	}

	return result
}

func getBuckets(client *s3.Client) ([]Bucket, error) {
	var buckets []Bucket
	output, err := client.ListBuckets(context.Background(), &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	for _, b := range output.Buckets {
		region := ""
		arn := fmt.Sprintf("arn:aws:s3:::%s", *b.Name)

		bucket := Bucket{
			BucketName: *b.Name,
			Arn:        arn,
			Region:     region,
			Policies:   []BucketPolicy{},
		}

		log.Printf("      - Bucket: %s\n", bucket.BucketName)

		// Get bucket location
		location, err := client.GetBucketLocation(context.Background(), &s3.GetBucketLocationInput{
			Bucket: aws.String(bucket.BucketName),
		})
		if err == nil {
			if location.LocationConstraint != "" {
				bucket.Region = string(location.LocationConstraint)
			} else {
				bucket.Region = "us-east-1"
			}
		}

		// Get bucket policy
		policy, err := client.GetBucketPolicy(context.Background(), &s3.GetBucketPolicyInput{
			Bucket: aws.String(bucket.BucketName),
		})
		if err == nil && policy.Policy != nil {
			var policyDoc map[string]interface{}
			if err := json.Unmarshal([]byte(*policy.Policy), &policyDoc); err == nil {
				policyDocument := PolicyDocument{}
				if version, ok := policyDoc["Version"].(string); ok {
					policyDocument.Version = version
				}
				if statements, ok := policyDoc["Statement"].([]interface{}); ok {
					policyDocument.Statement = statements
				}

				bucketPolicy := BucketPolicy{
					PolicyName:     "BucketPolicy",
					PolicyDocument: policyDocument,
				}
				bucket.Policies = append(bucket.Policies, bucketPolicy)
				log.Printf("        - Policy: %s\n", bucketPolicy.PolicyName)
			}
		}

		// Get versioning status
		versioning, err := client.GetBucketVersioning(context.Background(), &s3.GetBucketVersioningInput{
			Bucket: aws.String(bucket.BucketName),
		})
		if err == nil {
			bucket.VersioningEnabled = versioning.Status == "Enabled"
		}

		// Get object lock configuration
		objectLock, err := client.GetObjectLockConfiguration(context.Background(), &s3.GetObjectLockConfigurationInput{
			Bucket: aws.String(bucket.BucketName),
		})
		if err == nil && objectLock.ObjectLockConfiguration != nil {
			config := objectLock.ObjectLockConfiguration
			lockConfig := &ObjectLockConfiguration{
				Enabled: config.ObjectLockEnabled == "Enabled",
			}
			if config.Rule != nil && config.Rule.DefaultRetention != nil {
				retention := config.Rule.DefaultRetention
				lockConfig.Mode = string(retention.Mode)
				if retention.Days != nil {
					lockConfig.Retention = fmt.Sprintf("%d days", *retention.Days)
				}
				if retention.Years != nil {
					lockConfig.Retention = fmt.Sprintf("%d years", *retention.Years)
				}
			}
			bucket.ObjectLockConfiguration = lockConfig
		}

		// Get public access block configuration
		publicAccess, err := client.GetPublicAccessBlock(context.Background(), &s3.GetPublicAccessBlockInput{
			Bucket: aws.String(bucket.BucketName),
		})
		if err == nil && publicAccess.PublicAccessBlockConfiguration != nil {
			pab := publicAccess.PublicAccessBlockConfiguration
			blockPublicAcls := false
			if pab.BlockPublicAcls != nil {
				blockPublicAcls = *pab.BlockPublicAcls
			}
			ignorePublicAcls := false
			if pab.IgnorePublicAcls != nil {
				ignorePublicAcls = *pab.IgnorePublicAcls
			}
			blockPublicPolicy := false
			if pab.BlockPublicPolicy != nil {
				blockPublicPolicy = *pab.BlockPublicPolicy
			}
			restrictPublicBuckets := false
			if pab.RestrictPublicBuckets != nil {
				restrictPublicBuckets = *pab.RestrictPublicBuckets
			}
			bucket.PublicAccessBlock = &PublicAccessBlock{
				BlockPublicAcls:       blockPublicAcls,
				IgnorePublicAcls:      ignorePublicAcls,
				BlockPublicPolicy:     blockPublicPolicy,
				RestrictPublicBuckets: restrictPublicBuckets,
			}
		}

		buckets = append(buckets, bucket)
	}

	return buckets, nil
}
