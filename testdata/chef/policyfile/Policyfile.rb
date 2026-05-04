name "myapp"

default_source :supermarket

run_list "recipe[apt]", "recipe[nginx]"

cookbook "apt", "~> 7.0"
cookbook "nginx", "~> 11.0"
