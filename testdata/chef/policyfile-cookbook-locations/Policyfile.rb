name 'locations'
run_list 'recipe[my_app]'
default_source :supermarket
cookbook 'my_app', path: 'cookbooks/my_app'
cookbook 'mysql', github: 'opscode-cookbooks/mysql', branch: 'master'
cookbook 'chef-ingredient', git: 'https://github.com/chef-cookbooks/chef-ingredient.git', tag: 'v0.12.0'
