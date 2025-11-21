export OS_AUTH_URL=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-authURL.txt');
export OS_PROJECT_DOMAIN_NAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-domainName.txt');
export OS_USER_DOMAIN_NAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-domainName.txt');
export OS_REGION_NAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-region.txt');
export OS_AUTH_STRATEGY=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-authStrategy.txt');
export OS_TENANT_NAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-tenantName.txt');
export OS_USERNAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-username.txt');
export OS_PASSWORD=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-password.txt');
export OS_AUTH_TYPE=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-authType.txt');
export OS_APPLICATION_CREDENTIAL_ID=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-applicationCredentialID.txt');
export OS_APPLICATION_CREDENTIAL_NAME=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-applicationCredentialName.txt');
export OS_APPLICATION_CREDENTIAL_SECRET=$(< 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-applicationCredentialSecret.txt');
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-applicationCredentialID.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-applicationCredentialName.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-applicationCredentialSecret.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-authStrategy.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-authType.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-authURL.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-domainName.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-password.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-region.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-tenantName.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-username.txt';

# Run this command to configure openstack for your shell:
# eval $(gardenctl provider-env bash)
