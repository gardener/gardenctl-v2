export GOOGLE_CREDENTIALS='{"client_email":"test@example.org","project_id":"test"}';
export GOOGLE_CREDENTIALS_ACCOUNT="test@example.org";
export CLOUDSDK_CORE_PROJECT="test";
export CLOUDSDK_COMPUTE_REGION="europe";
gcloud auth activate-service-account $GOOGLE_CREDENTIALS_ACCOUNT --key-file <(printf "%s" "$GOOGLE_CREDENTIALS");
printf 'Successfully configured the "gcp" CLI for your current shell session.\nRun the following command to reset this configuration:\n%s\n' '$ eval $(gardenctl cloud-env --garden test --seed seed --shoot shoot -u bash)';

# Run this command to configure the "gcloud" CLI for your shell:
# eval $(gardenctl cloud-env bash)
