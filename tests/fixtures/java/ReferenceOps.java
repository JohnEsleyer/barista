public final class ReferenceOps {
    private final long id;

    public ReferenceOps(long id) {
        this.id = id;
    }

    public long getId() {
        return id;
    }

    public boolean same(Object other) {
        return this == other;
    }

    public static int ternary(boolean flag, int a, int b) {
        return flag ? a : b;
    }

    public boolean isReferenceOps(Object other) {
        return other instanceof ReferenceOps;
    }

    public long castId(Object other) {
        ReferenceOps typed = (ReferenceOps) other;
        return typed.id;
    }

    public int compare(long left, long right) {
        if (left < right) {
            return -1;
        }
        if (left > right) {
            return 1;
        }
        return 0;
    }

    public static int mask(int value) {
        if ((value & 2) != 0) {
            return 7;
        }
        return 9;
    }

    public static long sumLongs(long[] values) {
        long total = 0;
        for (int i = 0; i < values.length; i++) {
            total += values[i];
        }
        return total;
    }

    public static long[] makeLongs(long first, long second) {
        long[] values = new long[2];
        values[0] = first;
        values[1] = second;
        return values;
    }
}
