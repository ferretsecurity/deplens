package com.example;

import org.springframework.web.bind.annotation.*;
import java.util.List;

@RestController
@RequestMapping("/users")
public class UserController {

    @GetMapping
    public List<String> listUsers() {
        return List.of();
    }

    @PostMapping
    public String createUser(@RequestBody String body) {
        return "new";
    }

    @GetMapping("/{id}")
    public String getUser(@PathVariable String id) {
        return id;
    }
}
