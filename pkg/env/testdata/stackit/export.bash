export OS_AUTH_URL=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-authURL.txt');
export OS_PROJECT_DOMAIN_NAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-domainName.txt');
export OS_USER_DOMAIN_NAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-domainName.txt');
export OS_REGION_NAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-region.txt');
export OS_AUTH_STRATEGY=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-authStrategy.txt');
export OS_TENANT_NAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-tenantName.txt');
export OS_USERNAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-username.txt');
export OS_PASSWORD=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-password.txt');
export STACKIT_PROJECT_ID=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-projectId.txt');
export STACKIT_REGION=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-stackitRegion.txt');
STACKIT_CLI_PROFILE=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-stackitCliProfile.txt');
stackit config profile create --no-set --empty --ignore-existing -- "${STACKIT_CLI_PROFILE}";
export STACKIT_CLI_PROFILE;
stackit auth activate-service-account --service-account-key-path 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-serviceaccount.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-authStrategy.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-authURL.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-domainName.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-password.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-projectId.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-region.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-serviceaccount.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-stackitCliProfile.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-stackitRegion.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-tenantName.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/PLACEHOLDER_HASH-username.txt';

# Run this command to configure stackit for your shell:
# eval $(gardenctl provider-env bash)
