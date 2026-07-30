name 'composed'
run_list 'recipe[application]'
default_source :supermarket
include_policy 'base', path: './base.lock.json'
include_policy 'directory-base', path: '.'
include_policy 'shared', git: 'https://git.example.test/example/shared', path: 'shared.lock.json'
include_policy 'remote', policy_revision_id: 'revision1', remote: 'https://internal.example.test/remote.lock.json'
include_policy 'server-base', policy_name: 'base', policy_revision_id: 'revision1', server: 'https://chef-server.example.test/organizations/example'
cookbook 'application', '~> 4.0'
