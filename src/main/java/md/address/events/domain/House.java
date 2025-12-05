package md.address.events.domain;

import java.util.List;

public record House(
        List<String> numbers,
        List<List<String>> ranges
) {
}
