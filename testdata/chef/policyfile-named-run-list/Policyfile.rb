name 'application'
run_list 'recipe[application::default]'
named_run_list :update_app, 'application::update', 'recipe[application::restart]'
default_source :supermarket
cookbook 'application', '~> 4.0'
