/**
 * Kotlin fixture for Phase 4: null checks, data classes, and default
 * parameters.
 *
 * Compile with:
 *   kotlinc tests/fixtures/kotlin/Greeting.kt -d tests/fixtures/out
 */
data class User(val id: Long, val name: String)

fun greet(name: String): String = "Hello, $name!"

fun farewell(name: String, punctuation: String = "!"): String =
    "Goodbye, $name$punctuation"

fun main() {
    println(greet("Barista"))
}
