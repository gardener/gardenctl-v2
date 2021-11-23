export GOOGLE_CREDENTIALS='{"client_email":"test@example.org","project_id":"test"}';
export GOOGLE_CREDENTIALS_ACCOUNT="test@example.org";
export CLOUDSDK_CORE_PROJECT="test";
export CLOUDSDK_COMPUTE_REGION="europe";
gcloud auth activate-service-account $GOOGLE_CREDENTIALS_ACCOUNT --key-file <(printf "%s" "$GOOGLE_CREDENTIALS");
printf 'Successfully configured the "gcloud" CLI for your current shell session.\n\n# Run the following command to reset this configuration:\n# eval $(gardenctl cloud-env --garden test --project project --shoot shoot -u bash)\n';

# Run this command to configure the "gcloud" CLI for your shell:
# eval $(gardenctl cloud-env bash)
