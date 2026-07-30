(defproject example/managed-usage "0.1.0"
  :managed-dependencies [[org.slf4j/slf4j-api "2.0.13"]
                         [com.example/testing "1.0.0" :classifier "tests"]]
  :dependencies [[org.slf4j/slf4j-api :exclusions [org.slf4j/slf4j-simple]]
                 [com.example/testing nil :classifier "tests"]])
