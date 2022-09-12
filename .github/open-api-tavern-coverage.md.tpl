<!-- This file is templated with https://pkg.go.dev/html/template -->

# Open-API Tavern Coverage Report
<table>
	<tbody>
		<tr>
			<td>Endpoint</td>
			<td>Method</td>
			<td>Test Case Count</td>
			<td>Status</td>
		</tr>
{{- range $endpoint := .endpoints }}
    <tr>
        <td>{{$endpoint.url}}</td>
        <td>{{$endpoint.method}}</td>
        <td>{{$endpoint.count}}</td>
        {{- if eq $endpoint.count 0 }}
			<td>:x:</td>
        {{- end}}
        {{- if eq $endpoint.count 1 }}
			<td>:warning:</td>
        {{- end}}
        {{- if gt $endpoint.count 1 }}
			<td>:white_check_mark:</td>
        {{- end}}
    </tr>
{{- end}}
	</tbody>
</table>