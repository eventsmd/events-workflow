package md.address.events.events;

import com.fasterxml.jackson.annotation.JsonInclude;

import java.util.List;

@JsonInclude(JsonInclude.Include.NON_NULL)
public record EventAddress(KladrRef region, KladrRef city, KladrRef street, List<String> houses) {
}
