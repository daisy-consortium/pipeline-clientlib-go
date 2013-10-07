package pipeline

import (
	"fmt"
	"github.com/capitancambio/restclient"
	"io"
	"log"
)

//Available api entry names
const (
	API_ALIVE        = "alive"
	API_SCRIPT       = "script"
	API_SCRIPTS      = "scripts"
	API_JOBREQUEST   = "jobRequest"
	API_JOB          = "job"
	API_JOBS         = "jobs"
	API_DEL_JOB      = "del_job"
	API_RESULT       = "results"
	API_LOG          = "log"
	API_HALT         = "halt"
	API_CLIENTS      = "clients"
	API_NEWCLIENT    = "new_client"
	API_CLIENT       = "client"
	API_DELETECLIENT = "delete_client"
	API_MODIFYCLIENT = "modify_client"
	API_PROPERTIES   = "properties"
)

//Defines the information for an api entry
type apiEntry struct {
	urlPath  string
	method   string
	okStatus int
}

//Available api entries
var apiEntries = map[string]apiEntry{
	API_ALIVE:        apiEntry{"alive", "GET", 200},
	API_SCRIPTS:      apiEntry{"scripts", "GET", 200},
	API_SCRIPT:       apiEntry{"scripts/%v", "GET", 200},
	API_JOBREQUEST:   apiEntry{"jobs", "POST", 201},
	API_JOB:          apiEntry{"jobs/%v?msgSeq=%v", "GET", 200},
	API_DEL_JOB:      apiEntry{"jobs/%v", "DELETE", 204},
	API_RESULT:       apiEntry{"jobs/%v/result", "GET", 200},
	API_JOBS:         apiEntry{"jobs", "GET", 200},
	API_LOG:          apiEntry{"jobs/%v/log", "GET", 200},
	API_HALT:         apiEntry{"admin/halt/%v", "GET", 204},
	API_CLIENTS:      apiEntry{"admin/clients", "GET", 200},
	API_NEWCLIENT:    apiEntry{"admin/clients", "POST", 201},
	API_CLIENT:       apiEntry{"admin/clients/%v", "GET", 200},
	API_DELETECLIENT: apiEntry{"admin/clients/%v", "DELETE", 204},
	API_MODIFYCLIENT: apiEntry{"admin/clients/%v", "PUT", 200},
	API_PROPERTIES:   apiEntry{"admin/properties", "GET", 200},
}

//Pipeline struct stores different configuration paramenters
//for the communication with the pipeline framework
type Pipeline struct {
	BaseUrl       string                                //baseurl of the framework
	clientMaker   func() doer                           //client to perform the rest queries
	authenticator func(req *restclient.RequestResponse) //authentication function
}

func NewPipeline(baseUrl string) *Pipeline {
	return &Pipeline{
		BaseUrl:       baseUrl,
		authenticator: func(*restclient.RequestResponse) {},
		clientMaker:   newClient,
	}
}

func (p *Pipeline) SetCredentials(clientKey, clientSecret string) {
	p.authenticator = authenticator(clientKey, clientSecret)
}

func (p *Pipeline) SetUrl(url string) {
	p.BaseUrl = url
}

//Returns a simple string representation of the Alive struct in the format:
//Alive:[#authentication:value #mode:value #version:value]
func (a Alive) String() string {
	return fmt.Sprintf("Alive:[#authentication:%v #mode:%v #version:%v]", a.Authentication, a.Mode, a.Version)
}

//Calls the alive api entry
//TODO link to wiki
func (p Pipeline) Alive() (alive Alive, err error) {
	req := p.newResquest(API_ALIVE, &alive, nil)
	_, err = p.do(req, defaultErrorHandler())
	return
}

//List of scripts

//Returns the list of available scripts
func (p Pipeline) Scripts() (scripts Scripts, err error) {
	req := p.newResquest(API_SCRIPTS, &scripts, nil)
	_, err = p.do(req, defaultErrorHandler())
	return
}

//Returns the list of available scripts
func (p Pipeline) Script(id string) (script Script, err error) {
	req := p.newResquest(API_SCRIPT, &script, nil, id)
	_, err = p.do(req, errorHandler(map[int]string{404: "Script " + id + " not found"}))
	return
}

//Returns the url for a given script id
func (p Pipeline) ScriptUrl(id string) string {
	//This should call the server, but it just would add more overhead
	//so it's computed here
	req := p.newResquest(API_SCRIPT, nil, nil, id)
	return req.Url
}

//Overrides the xml decoder to get raw data
func multipartResultClientMaker(p Pipeline) func() doer {
	return func() doer {
		cli := p.clientMaker()
		//change the default encodersuppier by the multipart
		cli.SetEncoderSupplier(func(r io.Writer) restclient.Encoder {
			return NewMultipartEncoder(r)
		})
		cli.SetContentType("multipart/form-data; boundary=" + boundary)
		return cli
	}
}

//Specific multipart request
func buildMultipartReq(jobReq JobRequest, data []byte) *MultipartData {
	return &MultipartData{
		data:    RawData{&data},
		request: jobReq,
	}
}

//Sends a JobRequest to the server
func (p Pipeline) JobRequest(newJob JobRequest, data []byte) (job Job, err error) {
	var reqData interface{} = &newJob
	log.Println("data len request ", len(data))
	//check if we have data
	if len(data) > 0 {
		log.Println("Sending multipart job request")
		p.clientMaker = multipartResultClientMaker(p)
		reqData = buildMultipartReq(newJob, data)
	}
	log.Println("Sending job request")
	log.Println(newJob.Script.Id)
	req := p.newResquest(API_JOBREQUEST, &job, reqData)
	_, err = p.do(req, errorHandler(map[int]string{
		400: "Job request is not valid",
	}))
	return
}

//Sends a Job query to the webservice
func (p Pipeline) Job(id string, messageSequence int) (job Job, err error) {
	req := p.newResquest(API_JOB, &job, nil, id, messageSequence)
	_, err = p.do(req, errorHandler(map[int]string{
		404: "Job " + id + " not found",
	}))
	return
}

//Sends a request to the server in order to get all the jobs
func (p Pipeline) Jobs() (jobs Jobs, err error) {
	req := p.newResquest(API_JOBS, &jobs, nil)
	_, err = p.do(req, defaultErrorHandler())
	return
}

//Deletes a job
func (p Pipeline) DeleteJob(id string) (ok bool, err error) {
	req := p.newResquest(API_DEL_JOB, nil, nil, id)
	_, err = p.do(req, errorHandler(map[int]string{
		404: "Job " + id + " not found",
	}))
	if err == nil {
		ok = true
	}
	return
}

//Overrides the xml decoder to get raw data
func resultClientMaker(p Pipeline) func() doer {
	return func() doer {
		cli := p.clientMaker()
		cli.SetDecoderSupplier(func(r io.Reader) restclient.Decoder {
			return NewRawDataDecoder(r)
		})
		return cli
	}
}

//Returns the results of the job as an array of bytes
func (p Pipeline) Results(id string) (data []byte, err error) {
	//override the client maker
	p.clientMaker = resultClientMaker(p)
	rd := &RawData{Data: new([]byte)}
	req := p.newResquest(API_RESULT, rd, nil, id)
	_, err = p.do(req, errorHandler(map[int]string{
		404: "Job " + id + " not found",
	}))
	if err != nil {
		return nil, err
	}
	return *(rd.Data), nil
}

//Gets the log file for a job
func (p Pipeline) Log(id string) (data []byte, err error) {
	p.clientMaker = resultClientMaker(p)
	rd := &RawData{Data: new([]byte)}
	req := p.newResquest(API_LOG, rd, nil, id)
	_, err = p.do(req, errorHandler(map[int]string{
		404: "Job " + id + " not found",
	}))
	if err != nil {
		return nil, err
	}
	return *(rd.Data), nil
}

//Admin api
//Halts the ws
func (p Pipeline) Halt(key string) error {
	//override the client maker
	req := p.newResquest(API_HALT, nil, nil, key)
	_, err := p.do(req, defaultErrorHandler())
	return err
}

func (p Pipeline) Clients() (clients []Client, err error) {
	clientsStr := Clients{}
	req := p.newResquest(API_CLIENTS, &clientsStr, nil)
	_, err = p.do(req, defaultErrorHandler())
	if err != nil {
		return
	}
	clients = clientsStr.Clients
	return
}

func (p Pipeline) NewClient(in Client) (out Client, err error) {
	req := p.newResquest(API_NEWCLIENT, &out, &in)
	_, err = p.do(req, errorHandler(map[int]string{
		400: fmt.Sprintf("Client with id %v may already exist", in.Id),
	}))
	return
}

func (p Pipeline) Client(id string) (out Client, err error) {
	req := p.newResquest(API_CLIENT, &out, nil, id)
	_, err = p.do(req, errorHandler(map[int]string{
		404: "Client with id " + id + " not found",
	}))
	return
}

func (p Pipeline) DeleteClient(id string) (ok bool, err error) {
	req := p.newResquest(API_DELETECLIENT, nil, nil, id)
	_, err = p.do(req, errorHandler(map[int]string{
		404: "Client with id " + id + " not found",
	}))
	if err != nil {
		ok = true
	}
	return
}

func (p Pipeline) ModifyClient(in Client, id string) (out Client, err error) {
	req := p.newResquest(API_MODIFYCLIENT, &out, &in, id)
	_, err = p.do(req, errorHandler(map[int]string{
		404: "Client with id " + id + " not found",
	}))
	return
}

func (p Pipeline) Properties() (out []Property, err error) {
	props := Properties{}
	req := p.newResquest(API_PROPERTIES, &props, nil)
	_, err = p.do(req, defaultErrorHandler())
	if err != nil {
		return
	}
	return props.Properties, nil
}
