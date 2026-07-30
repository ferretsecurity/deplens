name 'attributes'
run_list 'recipe[mysql]'
default_source :supermarket
cookbook 'mysql', '~> 8.0'
default['stage']['mysql']['install_s3'] = 'https://s3.example.test/stage.rpm'
default['prod']['mysql']['install_s3'] = 'https://s3.example.test/prod.rpm'
override['mysql']['server_root_password'] = 'policy-secret'
