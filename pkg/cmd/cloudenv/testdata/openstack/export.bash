export OS_IDENTITY_API_VERSION="3";
export OS_AUTH_VERSION="3";
export OS_AUTH_STRATEGY="keystone";
export OS_AUTH_URL="keyStoneURL";
export OS_TENANT_NAME="tenant";
export OS_PROJECT_DOMAIN_NAME="domain";
export OS_USER_DOMAIN_NAME="domain";
export OS_USERNAME="user";
export OS_PASSWORD="secret";
export OS_REGION_NAME="europe";
printf 'Successfully configured the "openstack" CLI for your current shell session.\nRun the following command to reset this configuration:\n%s\n' '$ eval $(gardenctl cloud-env --garden test --project project --shoot shoot -u bash)';

# Run this command to configure the "openstack" CLI for your shell:
# eval $(gardenctl cloud-env bash)
