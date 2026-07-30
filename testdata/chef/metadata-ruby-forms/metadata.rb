name 'ruby_forms'
maintainer 'Example'
license 'Apache-2.0'
description 'Ruby DSL declaration forms'
version '1.0.0'

depends("apt", "~> 7.0")
%w[nginx redis].each { |cookbook| depends cookbook }
