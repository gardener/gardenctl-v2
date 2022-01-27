set -gx AZURE_CLIENT_ID 'client-id';
set -gx AZURE_CLIENT_SECRET 'client-secret';
set -gx AZURE_TENANT_ID 'tenant-id';
set -gx AZURE_SUBSCRIPTION_ID 'subscription-id';
set -gx AZURE_CONFIG_DIR 'session-dir/.config/az';
az login --service-principal --username "$AZURE_CLIENT_ID" --password "$AZURE_CLIENT_SECRET" --tenant "$AZURE_TENANT_ID";
az account set --subscription "$AZURE_SUBSCRIPTION_ID";
printf 'Run the following command to log out and remove access to Azure subscriptions:\n$ eval (gardenctl provider-env --garden test --project project --shoot shoot -u fish)\n';

# Run this command to configure az for your shell:
# eval (gardenctl provider-env fish)
