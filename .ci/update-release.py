#!/usr/bin/env python3

from dataclasses import dataclass
import pathlib
import util

from github.util import GitHubRepositoryHelper

@dataclass
class Binary:
    path: str
    name: str

OUTPUT_FILE_BINARIES = [
    Binary(
        path='darwin-amd64',
        name='gardenctl_v2_darwin_amd64'
    ),
    Binary(
        path='linux-amd64',
        name='gardenctl_v2_linux_amd64'
    ),
]
VERSION_FILE_NAME='VERSION'

repo_owner_and_name = util.check_env('SOURCE_GITHUB_REPO_OWNER_AND_NAME')
repo_dir = util.check_env('MAIN_REPO_DIR')
output_dir = util.check_env('BINARY_PATH')

repo_owner, repo_name = repo_owner_and_name.split('/')

version_file_contents = version_file_path.read_text()

cfg_factory = util.ctx().cfg_factory()
github_cfg = cfg_factory.github('github_com')

github_repo_helper = GitHubRepositoryHelper(
    owner=repo_owner,
    name=repo_name,
    github_cfg=github_cfg,
)

gh_release = github_repo_helper.repository.release_from_tag(version_file_contents)

repo_path = pathlib.Path(repo_dir).resolve()
output_path = pathlib.Path(output_dir).resolve()
version_file_path = repo_path / VERSION_FILE_NAME
for binary in OUTPUT_FILE_BINARIES:
    output_file_path = output_path / binary.path / binary.name

    gh_release.upload_asset(
        content_type='application/octet-stream',
        name=f'{bin.name}-{version_file_contents}',
        asset=output_file_path.open(mode='rb'),
    )
