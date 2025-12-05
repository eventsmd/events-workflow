package md.address.events.geo;

import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.service.annotation.GetExchange;

import java.util.List;

public interface AddressApi {

    @GetExchange("/parse")
    List<AddressKladr> find(@RequestParam("address") String address);
}
