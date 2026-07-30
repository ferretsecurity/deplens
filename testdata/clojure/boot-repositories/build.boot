(set-env!
 :repositories #(conj % ["private" {:url "https://repo.example.test/maven/"
                                     :username "ci"
                                     :password "not-a-real-secret"}])
 :dependencies '[[com.cemerick/piggieback "0.2.2"]
                 [weasel "0.7.0"]])
