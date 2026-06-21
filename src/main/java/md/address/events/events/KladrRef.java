package md.address.events.events;

import com.fasterxml.jackson.annotation.JsonInclude;

@JsonInclude(JsonInclude.Include.NON_NULL)
public record KladrRef(String name, String kladr, String type) {
}
