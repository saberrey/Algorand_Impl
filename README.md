# What's this?
Go implementation of Algorand based on https://github.com/ericderegt/algorand
# How to test it?
cd server  
./server peernumber 50(on my own PC, it can only hold 100 nodes, get a more powerful CPU for more nodes) seed "Algorand" algorand 8000  
  
if you want to send some transactions, run:  
cd ..  
cd client  
./client 127.0.0.1:6001  
  
it will send 1000 transactions to the network, you can see the consensus result in client, have fun :)  
