package dpfm_api_caller

import (
	"context"
	dpfm_api_input_reader "data-platform-api-freight-order-deletes-rmq-kube/DPFM_API_Input_Reader"
	dpfm_api_output_formatter "data-platform-api-freight-order-deletes-rmq-kube/DPFM_API_Output_Formatter"
	"data-platform-api-freight-order-deletes-rmq-kube/config"

	"github.com/latonaio/golang-logging-library-for-data-platform/logger"
	database "github.com/latonaio/golang-mysql-network-connector"
	rabbitmq "github.com/latonaio/rabbitmq-golang-client-for-data-platform"
	"golang.org/x/xerrors"
)

type DPFMAPICaller struct {
	ctx  context.Context
	conf *config.Conf
	rmq  *rabbitmq.RabbitmqClient
	db   *database.Mysql
}

func NewDPFMAPICaller(
	conf *config.Conf, rmq *rabbitmq.RabbitmqClient, db *database.Mysql,
) *DPFMAPICaller {
	return &DPFMAPICaller{
		ctx:  context.Background(),
		conf: conf,
		rmq:  rmq,
		db:   db,
	}
}

func (c *DPFMAPICaller) AsyncDeletes(
	accepter []string,
	input *dpfm_api_input_reader.SDC,
	output *dpfm_api_output_formatter.SDC,
	log *logger.Logger,
) (interface{}, []error) {
	var response interface{}
	if input.APIType == "deletes" {
		response = c.deleteSqlProcess(input, output, accepter, log)
	}

	return response, nil
}

func (c *DPFMAPICaller) deleteSqlProcess(
	input *dpfm_api_input_reader.SDC,
	output *dpfm_api_output_formatter.SDC,
	accepter []string,
	log *logger.Logger,
) *dpfm_api_output_formatter.Message {
	var headerData *dpfm_api_output_formatter.Header
	itemData := make([]dpfm_api_output_formatter.Item, 0)
	for _, a := range accepter {
		switch a {
		case "Header":
			h, i := c.headerDelete(input, output, log)
			headerData = h
			if h == nil || i == nil {
				continue
			}
			itemData = append(itemData, *i...)
		case "Item":
			i := c.itemDelete(input, output, log)
			if i == nil {
				continue
			}
			itemData = append(itemData, *i...)
		}
	}

	return &dpfm_api_output_formatter.Message{
		Header: headerData,
		Item:   &itemData,
	}
}

func (c *DPFMAPICaller) headerDelete(
	input *dpfm_api_input_reader.SDC,
	output *dpfm_api_output_formatter.SDC,
	log *logger.Logger,
) *dpfm_api_output_formatter.Header {
	sessionID := input.RuntimeSessionID
	header := c.HeaderRead(input, log)
	header.FreightOrder = input.Header.FreightOrder
	header.IsMarkedForDeletion = input.Header.IsMarkedForDeletion
	res, err := c.rmq.SessionKeepRequest(nil, c.conf.RMQ.QueueToSQL()[0], map[string]interface{}{"message": header, "function": "FreightOrderHeader", "runtime_session_id": sessionID})
	if err != nil {
		err = xerrors.Errorf("rmq error: %w", err)
		log.Error("%+v", err)
		return nil
	}
	res.Success()
	if !checkResult(res) {
		output.SQLUpdateResult = getBoolPtr(false)
		output.SQLUpdateError = "Header Data cannot delete"
		return nil
	}

	return header
}

func checkResult(msg rabbitmq.RabbitmqMessage) bool {
	data := msg.Data()
	d, ok := data["result"]
	if !ok {
		return false
	}
	result, ok := d.(string)
	if !ok {
		return false
	}
	return result == "success"
}

func getBoolPtr(b bool) *bool {
	return &b
}
