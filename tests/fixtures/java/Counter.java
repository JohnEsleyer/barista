/**
 * Phase 2 object fixture: instance fields, a constructor, static state, and
 * all three supported dispatch forms.
 *
 * Compile with:
 *   javac -d tests/fixtures/out tests/fixtures/java/Counter.java
 */
public final class Counter {
    private int value;
    private static int created = 2;

    public Counter(int initial) {
        this.value = initial;
        created++;
    }

    public int increment() {
        value = value + 1;
        return value;
    }

    public int getValue() {
        return value;
    }

    public int addValue(int delta) {
        return increment() + delta;
    }

    public static Counter create(int initial) {
        return new Counter(initial);
    }

    public static int createAndGet(int initial) {
        Counter counter = create(initial);
        return counter.getValue();
    }

    public static int getCreated() {
        return created;
    }
}
