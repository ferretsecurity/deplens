(set-env!
 :dependencies #(conj % '[example/private-lib "1.2.3"
                           :exclusions [org.clojure/clojure]
                           :classifier "jdk8"
                           :extension "jar"]))
