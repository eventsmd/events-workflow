package md.address.events.geo;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.client.RestClient;
import org.springframework.web.client.support.RestClientAdapter;
import org.springframework.web.service.invoker.HttpServiceProxyFactory;

@Configuration
public class GeoConfiguration {

    @Value("${GEO_BASE_URL:http://localhost:8081}")
    private String geoBaseUrl;

    @Bean
    public AddressApi addressApi() {

        var restClient = RestClient.builder()
                .baseUrl(geoBaseUrl)
                .build();

        var adapter = RestClientAdapter.create(restClient);
        var factory = HttpServiceProxyFactory.builderFor(adapter).build();
        return factory.createClient(AddressApi.class);
    }
}
