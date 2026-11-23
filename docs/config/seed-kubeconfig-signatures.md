# Seed kubeconfig signatures

For a non-managed seed, gardenctl loads the kubeconfig from the `garden/<seed>.login` Secret in the virtual Garden. It verifies the kubeconfig signature before parsing the kubeconfig. Managed seeds use the shoot kubeconfig flow.

## Configure a trusted public key

Obtain the ECDSA P-256 public key from your landscape administrator through an authenticated channel independent of the virtual Garden. Add the PEM `PUBLIC KEY` block (SubjectPublicKeyInfo) to the matching Garden by identity or alias. If you have the key in a file, use `--file`.

```sh
gardenctl config add-seed-key landscape-dev --file public-key.pem
```

You can also use `--key` to include the PEM public key in a single command for easy sharing. Enclose the complete PEM block in single quotes and keep its line breaks.

```sh
gardenctl config add-seed-key landscape-dev --key '-----BEGIN PUBLIC KEY-----
<public key from your landscape administrator>
-----END PUBLIC KEY-----'
```

Replace the placeholder with the contents of your public key file between the PEM header and footer. Run `gardenctl config add-seed-key --help` to see both input options.

The command appends the key to `seedKubeconfigPublicKeys` in the local configuration file (by default `~/.garden/gardenctl-v2.yaml`). You can also edit this list directly. A signature must verify with at least one key in the Garden's list.

```yaml
gardens:
  - identity: landscape-dev
    kubeconfig: /path/to/garden-kubeconfig.yaml
    seedKubeconfigPublicKeys:
      - |
        -----BEGIN PUBLIC KEY-----
        <public key from your landscape administrator>
        -----END PUBLIC KEY-----
```

## Sign a kubeconfig

Generate the kubeconfig from an approved source and sign it in a trusted environment. Use a distinct ECDSA P-256 key for each landscape. Keep the private key and signing permissions separate from the publishing pipeline.

To generate a file-based key pair, run these commands once in a private directory. Distribute `public-key.pem` to users.

```sh
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:prime256v1 -out signing-key.pem
openssl pkey -in signing-key.pem -pubout -out public-key.pem
```

Run `openssl dgst -sha256 kubeconfig.yaml` and copy the lowercase digest into `manifest.json`, keeping the `sha256:` prefix. Set `seedName` to the seed resource name. Use the four fields shown below.

```json
{
  "apiVersion": "gardenctl.gardener.cloud/v1alpha1",
  "kind": "SeedKubeconfigSignature",
  "seedName": "seed-a",
  "kubeconfigHash": "sha256:<lowercase hexadecimal digest>"
}
```

Sign the exact bytes of `manifest.json` with ECDSA P-256 and SHA-256. Store the signature as binary ASN.1 DER. For a file-based key, sign and verify with OpenSSL.

```sh
openssl dgst -sha256 -sign signing-key.pem -out manifest.sig manifest.json
openssl dgst -sha256 -verify public-key.pem -signature manifest.sig manifest.json
```

## Publish and verify the Secret

Give the publishing pipeline the three files below. Deploy them together as the `<seed>.login` Secret in the `garden` namespace of the virtual Garden, preserving the exact file bytes.

| Secret data key | Content after Kubernetes base64 decoding |
|---|---|
| `kubeconfig` | `kubeconfig.yaml` |
| `kubeconfig-signing-manifest` | `manifest.json` |
| `kubeconfig-signature` | `manifest.sig` |

Verify from a client configured with the trusted public key.

```sh
gardenctl kubeconfig --garden landscape-dev --seed seed-a --access-level=auto >/dev/null
```

## Rotate signing keys

Re-sign and publish whenever an approved kubeconfig changes. To rotate keys, add the new public key alongside the old one, publish Secrets signed with the new key, then remove the old key from each client's `seedKubeconfigPublicKeys` list.

Refresh affected shells to load the newly verified kubeconfig. In default symlink mode, repeat the `gardenctl target` command for the current target. With `linkKubeconfig: false`, run `gardenctl kubectl-env` again and apply its output in the shell.
