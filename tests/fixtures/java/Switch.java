public final class Switch {
    public static int dense(int x) {
        switch (x) {
            case 0: return 10;
            case 1: return 11;
            case 2: return 12;
            default: return -1;
        }
    }

    public static int sparse(int x) {
        switch (x) {
            case -100: return 1;
            case 7: return 2;
            case 1000: return 3;
            default: return 0;
        }
    }
}
