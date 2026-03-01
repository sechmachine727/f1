import os

from neuro_san.client.agent_session_factory import AgentSessionFactory
from neuro_san.client.streaming_input_processor import StreamingInputProcessor
from timedinput import timedinput


class DamageAgent:

    SESSION_TYPE = "direct"
    AGENT_NETWORK_NAME = "damage_engineer"
    THINKING_DIR = "logs/agents"
    THINKING_FILE = "damage_agent.txt"
    DEFAULT_INPUT = "DEFAULT"

    def __init__(self):
        factory = AgentSessionFactory()
        # Create log folders if they don't exist
        os.makedirs(self.THINKING_DIR, exist_ok=True)
        self.session = factory.create_session(session_type=self.SESSION_TYPE,
                                              agent_name=self.AGENT_NETWORK_NAME)
        # Initialize any conversation state here
        self.conversation_state = {
            "last_chat_response": None,
            "prompt": "Analyze the alerts log\n",
            "timeout": 5000.0,
            "num_input": 0,
            "user_input": None,
            "sly_data": None,
            "chat_filter": {"chat_filter_type": "MAXIMAL"},
        }

    def process_message(self, message):
        print(f"Received message: {message}")
        # Use the current session to process the input
        input_processor = StreamingInputProcessor(self.DEFAULT_INPUT,
                                                  self.THINKING_FILE,
                                                  self.session,
                                                  self.THINKING_DIR)
        # Update the conversation state with this turn's input
        self.conversation_state["user_input"] = message
        self.conversation_state = input_processor.process_once(self.conversation_state)
        # Get the agent response for this turn
        last_chat_response = self.conversation_state.get("last_chat_response")
        # print(f"*** Response: {last_chat_response}")
        return last_chat_response

if __name__ == "__main__":
    # Set env variables
    os.environ["AGENT_MANIFEST_FILE"] = "registries/manifest.hocon"
    # Instantiate the agent
    agent = DamageAgent()
    # Prompt user for input
    user_input = timedinput("Input message:\n",
                            timeout=60.0, # 1 minute
                            default="<===TIMEOUT===>")
    response = agent.process_message(user_input)
    print(f"Response:\n {response}")
