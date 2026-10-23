import barista.spring.annotation.RestController;
import barista.spring.annotation.RequestMapping;
import barista.spring.annotation.GetMapping;
import barista.spring.annotation.PostMapping;
import barista.spring.annotation.RequestBody;

class User {
    private long id;
    private String name;
    private String email;

    public User(long id, String name, String email) {
        this.id = id;
        this.name = name;
        this.email = email;
    }

    public long getId() { return id; }
    public String getName() { return name; }
    public String getEmail() { return email; }
}

class CreateUserRequest {
    private String name;
    private String email;

    public String getName() { return name; }
    public String getEmail() { return email; }
}

@RestController
@RequestMapping("/api/users")
public class UserController {

    @GetMapping("/current")
    public User getCurrentUser() {
        return new User(101L, "Alice Developer", "alice@example.com");
    }

    @PostMapping
    public User createUser(@RequestBody CreateUserRequest req) {
        return new User(102L, req.getName(), req.getEmail());
    }
}
