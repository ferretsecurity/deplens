%w{ apt database mysql osops-utils apache2 }.each do |dep|
  depends dep
end

depends "keystone", ">= 1.0.20"
