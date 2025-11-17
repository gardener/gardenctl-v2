set -gx AZURE_CLIENT_ID (cat -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-clientID.txt');
set -gx AZURE_TENANT_ID (cat -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-tenantID.txt');
set -gx AZURE_SUBSCRIPTION_ID (cat -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-subscriptionID.txt');
set -gx AZURE_CONFIG_DIR 'PLACEHOLDER_CONFIG_DIR';
set AZURE_CLIENT_SECRET (cat -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-clientSecret.txt');
az login --service-principal --username "$AZURE_CLIENT_ID" --password "$AZURE_CLIENT_SECRET" --tenant "$AZURE_TENANT_ID";
set -e AZURE_CLIENT_SECRET;
az account set --subscription "$AZURE_SUBSCRIPTION_ID";
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-clientID.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-clientSecret.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-region.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-subscriptionID.txt';
rm -f -- 'PLACEHOLDER_SESSION_DIR/provider-env/.PLACEHOLDER_HASH-tenantID.txt';
printf 'Run the following command to log out and remove access to Azure subscriptions:\n$ eval (gardenctl provider-env --garden test --project project --shoot shoot -u fish)\n';

# Run this command to configure az for your shell:
# eval (gardenctl provider-env fish)
