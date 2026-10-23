/**
 * Primitive arithmetic fixture for Phase 1.
 *
 * Compile with:
 *   javac -d tests/fixtures/out tests/fixtures/java/Calculator.java
 */
public final class Calculator {
    private Calculator() {
    }

    public static int add(int a, int b) {
        return a + b;
    }

    public static int subtract(int a, int b) {
        return a - b;
    }

    public static int multiply(int a, int b) {
        return a * b;
    }

    public static int divide(int a, int b) {
        return a / b;
    }

    public static int sumTo(int n) {
        int sum = 0;
        for (int i = 1; i <= n; i++) {
            sum += i;
        }
        return sum;
    }

    public static int absolute(int value) {
        if (value < 0) {
            return -value;
        }
        return value;
    }

    public static int max(int a, int b) {
        if (a > b) {
            return a;
        }
        return b;
    }
}
