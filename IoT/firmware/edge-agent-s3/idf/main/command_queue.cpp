#include "command_queue.h"
#include "freertos/FreeRTOS.h"
#include "freertos/queue.h"
#include "freertos/task.h"
#include <cstdlib>
#include <cstring>
namespace beefiscal::idf{
struct Owned{uint8_t*data;size_t size;Ingress ingress;};
struct CommandQueue::Impl{QueueHandle_t queue{};CommandSink sink{};void*context{};};
static void worker(void*raw){auto*self=static_cast<CommandQueue::Impl*>(raw);Owned item{};
  for(;;)if(xQueueReceive(self->queue,&item,portMAX_DELAY)==pdTRUE){
    self->sink({item.data,item.size,item.ingress},self->context);free(item.data);}}
esp_err_t CommandQueue::start(CommandSink sink,void*context,size_t depth){
  if(impl_||!sink||depth<1||depth>32)return ESP_ERR_INVALID_ARG;
  impl_=new Impl;
  impl_->sink=sink;impl_->context=context;impl_->queue=xQueueCreate(depth,sizeof(Owned));
  if(!impl_->queue)return ESP_ERR_NO_MEM;
  return xTaskCreate(worker,"intent-worker",8192,impl_,10,nullptr)==pdPASS?ESP_OK:ESP_ERR_NO_MEM;}
esp_err_t CommandQueue::enqueue(const CommandView&v){if(!impl_||!v.data||!v.size||v.size>8192)return ESP_ERR_INVALID_ARG;
  auto*copy=static_cast<uint8_t*>(malloc(v.size));if(!copy)return ESP_ERR_NO_MEM;memcpy(copy,v.data,v.size);
  Owned item{copy,v.size,v.ingress};if(xQueueSend(impl_->queue,&item,0)!=pdTRUE){free(copy);return ESP_ERR_NO_MEM;}return ESP_OK;}
esp_err_t queued_command_sink(const CommandView&v,void*context){auto*q=static_cast<CommandQueue*>(context);return q?q->enqueue(v):ESP_ERR_INVALID_STATE;}
}
