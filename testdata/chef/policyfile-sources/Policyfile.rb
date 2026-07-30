name 'source-precedence'
run_list 'recipe[chef-client]'
default_source :supermarket
default_source :supermarket, 'https://supermarket.example.test' do |s|
  s.preferred_for 'chef-client', 'nginx', 'mysql'
end
cookbook 'chef-client', '~> 18.0'
