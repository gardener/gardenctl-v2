set -gx AZURE_CLIENT_ID '12345678-1234-1234-1234-123456789012';
set -gx AZURE_TENANT_ID '87654321-4321-4321-4321-210987654321';
set -gx AZURE_SUBSCRIPTION_ID 'abcdef12-3456-7890-abcd-ef1234567890';
set -gx AZURE_CONFIG_DIR 'PLACEHOLDER_CONFIG_DIR';
set AZURE_CLIENT_SECRET 'AbCdE~fGhI.-jKlMnOpQrStUvWxYz0_123456789';
az login --service-principal --username "$AZURE_CLIENT_ID" --password "$AZURE_CLIENT_SECRET" --tenant "$AZURE_TENANT_ID";
set -e AZURE_CLIENT_SECRET;
az account set --subscription "$AZURE_SUBSCRIPTION_ID";
printf 'Run the following command to log out and remove access to Azure subscriptions:\n$ eval (gardenctl provider-env --garden test --project project --shoot shoot -u fish)\n';

# Run this command to configure az for your shell:
# eval (gardenctl provider-env fish)
